// Package main generates a JSON schema from configuration struct tags.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

type configField struct {
	Key          string
	Kind         string
	DefaultValue any
	Description  string
}

type dprintTag struct {
	DefaultValue any
}

const (
	kindUint32         = "uint32"
	kindBool           = "bool"
	defaultDraftSchema = "http://json-schema.org/draft-07/schema#"
)

func main() {
	var (
		dir               = flag.String("dir", ".", "target directory to scan")
		typeName          = flag.String("type", "", "target struct type name")
		outFile           = flag.String("out", "schema.json", "output file path")
		dprintTagKey      = flag.String("dprint-tag-key", "dprint", "struct tag key for resolver options")
		descriptionTagKey = flag.String("description-tag-key", "description", "struct tag key for field descriptions")
		schemaID          = flag.String("schema-id", "", "$id value for the generated schema")
		draftSchema       = flag.String("draft-schema", defaultDraftSchema, "$schema value for the generated schema")
		includeLocked     = flag.Bool("include-locked", false, "include the locked boolean property")
		lockedDescription = flag.String("locked-description", "", "description for locked property when include-locked is true")
	)
	flag.Parse()

	if *typeName == "" {
		exitWithError(fmt.Errorf("type name must not be empty"))
	}
	if *outFile == "" {
		exitWithError(fmt.Errorf("output file path must not be empty"))
	}
	if *schemaID == "" {
		exitWithError(fmt.Errorf("schema-id must not be empty"))
	}
	if *includeLocked && strings.TrimSpace(*lockedDescription) == "" {
		exitWithError(fmt.Errorf("locked-description must not be empty when include-locked is true"))
	}

	fields, err := parseStructFields(*dir, *typeName, *dprintTagKey, *descriptionTagKey)
	if err != nil {
		exitWithError(err)
	}

	source, err := renderSchemaJSON(*draftSchema, *schemaID, fields, *includeLocked, *lockedDescription)
	if err != nil {
		exitWithError(err)
	}

	if err := os.WriteFile(*outFile, source, 0o644); err != nil {
		exitWithError(err)
	}
}

func parseStructFields(
	dir string,
	typeName string,
	dprintTagKey string,
	descriptionTagKey string,
) ([]configField, error) {
	fset := token.NewFileSet()
	//nolint:staticcheck // ParseDir is enough for this local source generator.
	pkgs, err := parser.ParseDir(fset, dir, includeSourceFile, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory %q: %w", dir, err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			fields, ok, parseErr := parseStructTypeFromFile(file, typeName, dprintTagKey, descriptionTagKey)
			if parseErr != nil {
				return nil, parseErr
			}
			if ok {
				return fields, nil
			}
		}
	}

	return nil, fmt.Errorf("type %q not found", typeName)
}

func includeSourceFile(info fs.FileInfo) bool {
	name := info.Name()
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func parseStructTypeFromFile(
	file *ast.File,
	typeName string,
	dprintTagKey string,
	descriptionTagKey string,
) ([]configField, bool, error) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return nil, false, fmt.Errorf("type %q is not a struct", typeName)
			}

			fields, err := parseConfigFields(structType, dprintTagKey, descriptionTagKey)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse %q: %w", typeName, err)
			}
			return fields, true, nil
		}
	}

	return nil, false, nil
}

func parseConfigFields(
	structType *ast.StructType,
	dprintTagKey string,
	descriptionTagKey string,
) ([]configField, error) {
	fields := make([]configField, 0, len(structType.Fields.List))

	for _, field := range structType.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("each configuration field must declare exactly one name")
		}

		fieldName := field.Names[0].Name
		kind, err := parseSupportedKind(field.Type, fieldName)
		if err != nil {
			return nil, err
		}

		if field.Tag == nil {
			return nil, fmt.Errorf("field %q must define struct tags", fieldName)
		}
		tagText, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return nil, fmt.Errorf("field %q has invalid struct tags: %w", fieldName, err)
		}
		tags := reflect.StructTag(tagText)

		key, err := parseJSONKey(tags, fieldName)
		if err != nil {
			return nil, err
		}

		description, err := parseDescription(tags.Get(descriptionTagKey), fieldName, descriptionTagKey)
		if err != nil {
			return nil, err
		}

		parsedTag, err := parseDprintTag(tags.Get(dprintTagKey), kind, fieldName)
		if err != nil {
			return nil, err
		}

		fields = append(fields, configField{
			Key:          key,
			Kind:         kind,
			DefaultValue: parsedTag.DefaultValue,
			Description:  description,
		})
	}

	return fields, nil
}

func parseSupportedKind(expr ast.Expr, fieldName string) (string, error) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf("field %q must be bool or uint32", fieldName)
	}

	switch ident.Name {
	case kindUint32, kindBool:
		return ident.Name, nil
	default:
		return "", fmt.Errorf("field %q has unsupported type %q", fieldName, ident.Name)
	}
}

func parseJSONKey(tags reflect.StructTag, fieldName string) (string, error) {
	jsonTag := tags.Get("json")
	if jsonTag == "" {
		return "", fmt.Errorf("field %q must define a json tag", fieldName)
	}

	key := strings.TrimSpace(strings.Split(jsonTag, ",")[0])
	if key == "" || key == "-" {
		return "", fmt.Errorf("field %q has an invalid json tag key", fieldName)
	}

	return key, nil
}

func parseDescription(raw string, fieldName string, descriptionTagKey string) (string, error) {
	description := strings.TrimSpace(raw)
	if description == "" {
		return "", fmt.Errorf(
			"field %q must define a %q tag with a non-empty description",
			fieldName,
			descriptionTagKey,
		)
	}
	return description, nil
}

func parseDprintTag(raw string, kind string, fieldName string) (dprintTag, error) {
	if strings.TrimSpace(raw) == "" {
		return dprintTag{}, fmt.Errorf("field %q must define a dprint tag", fieldName)
	}

	parsed := dprintTag{}
	hasDefault := false

	for _, token := range strings.Split(raw, ",") {
		part := strings.TrimSpace(token)
		if part == "" {
			continue
		}

		if part == "global" {
			continue
		}

		if strings.HasPrefix(part, "default=") {
			if hasDefault {
				return dprintTag{}, fmt.Errorf("field %q has duplicate default options", fieldName)
			}
			hasDefault = true

			defaultText := strings.TrimSpace(strings.TrimPrefix(part, "default="))
			value, err := parseDefaultValue(defaultText, kind, fieldName)
			if err != nil {
				return dprintTag{}, err
			}
			parsed.DefaultValue = value
			continue
		}

		return dprintTag{}, fmt.Errorf("field %q has unknown dprint option %q", fieldName, part)
	}

	if !hasDefault {
		return dprintTag{}, fmt.Errorf("field %q must define default=... in dprint tag", fieldName)
	}

	return parsed, nil
}

func parseDefaultValue(value string, kind string, fieldName string) (any, error) {
	switch kind {
	case kindUint32:
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("field %q has invalid uint32 default %q", fieldName, value)
		}
		return parsed, nil
	case kindBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("field %q has invalid bool default %q", fieldName, value)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("field %q has unsupported type %q", fieldName, kind)
	}
}

func renderSchemaJSON(
	draftSchema string,
	schemaID string,
	fields []configField,
	includeLocked bool,
	lockedDescription string,
) ([]byte, error) {
	properties := make(map[string]*jsonschema.Schema, len(fields)+1)
	propertyOrder := make([]string, 0, len(fields)+1)

	if includeLocked {
		properties["locked"] = &jsonschema.Schema{
			Description: lockedDescription,
			Type:        "boolean",
		}
		propertyOrder = append(propertyOrder, "locked")
	}

	for _, field := range fields {
		property, err := toSchemaProperty(field)
		if err != nil {
			return nil, err
		}
		properties[field.Key] = property
		propertyOrder = append(propertyOrder, field.Key)
	}

	root := &jsonschema.Schema{
		Schema:        draftSchema,
		ID:            schemaID,
		Type:          "object",
		Properties:    properties,
		PropertyOrder: propertyOrder,
	}

	source, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON schema: %w", err)
	}

	return append(source, '\n'), nil
}

func toSchemaProperty(field configField) (*jsonschema.Schema, error) {
	defaultValue, err := encodeDefaultValue(field.DefaultValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encode default for %q: %w", field.Key, err)
	}

	switch field.Kind {
	case kindUint32:
		return &jsonschema.Schema{
			Description: field.Description,
			Default:     defaultValue,
			Type:        "integer",
			Minimum:     jsonschema.Ptr(0.0),
		}, nil
	case kindBool:
		return &jsonschema.Schema{
			Description: field.Description,
			Default:     defaultValue,
			Type:        "boolean",
		}, nil
	default:
		return nil, fmt.Errorf("unknown field kind %q", field.Kind)
	}
}

func encodeDefaultValue(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func exitWithError(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "gen-json-schema: %v\n", err)
	os.Exit(1)
}
