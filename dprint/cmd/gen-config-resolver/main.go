// Package main generates configuration resolver specs from struct tags.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type configField struct {
	FieldName           string
	Key                 string
	Kind                string
	DefaultValueLiteral string
	AllowGlobalOverride bool
}

type dprintTag struct {
	DefaultValueLiteral string
	AllowGlobalOverride bool
}

const (
	kindUint32 = "uint32"
	kindBool   = "bool"
)

func main() {
	var (
		dir            = flag.String("dir", ".", "target directory to scan")
		typeName       = flag.String("type", "", "target struct type name")
		outFile        = flag.String("out", "handler_config_generated.go", "output file path")
		specName       = flag.String("spec", "generatedConfigurationResolverSpec", "generated spec variable name")
		extraKnownKeys = flag.String("extra-known-keys", "", "additional known keys (comma-separated)")
		dprintTagKey   = flag.String("dprint-tag-key", "dprint", "struct tag key for resolver options")
	)
	flag.Parse()

	if *typeName == "" {
		exitWithError(fmt.Errorf("type name must not be empty"))
	}
	if *outFile == "" {
		exitWithError(fmt.Errorf("output file path must not be empty"))
	}
	if *specName == "" {
		exitWithError(fmt.Errorf("spec variable name must not be empty"))
	}

	pkgName, fields, err := parseStructFields(*dir, *typeName, *dprintTagKey)
	if err != nil {
		exitWithError(err)
	}

	knownKeys := mergeKnownKeys(fields, parseExtraKnownKeys(*extraKnownKeys))
	source, err := renderSource(pkgName, *typeName, *specName, fields, knownKeys)
	if err != nil {
		exitWithError(err)
	}

	formatted, err := format.Source(source)
	if err != nil {
		exitWithError(err)
	}

	if err := os.WriteFile(*outFile, formatted, 0o644); err != nil {
		exitWithError(err)
	}
}

func parseStructFields(dir string, typeName string, dprintTagKey string) (string, []configField, error) {
	fset := token.NewFileSet()
	//nolint:staticcheck // ParseDir is enough for this local source generator.
	pkgs, err := parser.ParseDir(fset, dir, includeSourceFile, parser.ParseComments)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse directory %q: %w", dir, err)
	}

	for pkgName, pkg := range pkgs {
		for _, file := range pkg.Files {
			fields, ok, err := parseStructTypeFromFile(file, typeName, dprintTagKey)
			if err != nil {
				return "", nil, err
			}
			if ok {
				return pkgName, fields, nil
			}
		}
	}

	return "", nil, fmt.Errorf("type %q not found", typeName)
}

func includeSourceFile(info fs.FileInfo) bool {
	name := info.Name()
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func parseStructTypeFromFile(file *ast.File, typeName string, dprintTagKey string) ([]configField, bool, error) {
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

			fields, err := parseConfigFields(structType, dprintTagKey)
			if err != nil {
				return nil, false, fmt.Errorf("failed to parse %q: %w", typeName, err)
			}
			return fields, true, nil
		}
	}

	return nil, false, nil
}

func parseConfigFields(structType *ast.StructType, dprintTagKey string) ([]configField, error) {
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

		parsedTag, err := parseDprintTag(tags.Get(dprintTagKey), kind, fieldName)
		if err != nil {
			return nil, err
		}

		fields = append(fields, configField{
			FieldName:           fieldName,
			Key:                 key,
			Kind:                kind,
			DefaultValueLiteral: parsedTag.DefaultValueLiteral,
			AllowGlobalOverride: parsedTag.AllowGlobalOverride,
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
			parsed.AllowGlobalOverride = true
			continue
		}

		if strings.HasPrefix(part, "default=") {
			if hasDefault {
				return dprintTag{}, fmt.Errorf("field %q has duplicate default options", fieldName)
			}
			hasDefault = true

			defaultText := strings.TrimSpace(strings.TrimPrefix(part, "default="))
			valueLiteral, err := parseDefaultValueLiteral(defaultText, kind, fieldName)
			if err != nil {
				return dprintTag{}, err
			}
			parsed.DefaultValueLiteral = valueLiteral
			continue
		}

		return dprintTag{}, fmt.Errorf("field %q has unknown dprint option %q", fieldName, part)
	}

	if !hasDefault {
		return dprintTag{}, fmt.Errorf("field %q must define default=... in dprint tag", fieldName)
	}

	return parsed, nil
}

func parseDefaultValueLiteral(value string, kind string, fieldName string) (string, error) {
	switch kind {
	case kindUint32:
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return "", fmt.Errorf("field %q has invalid uint32 default %q", fieldName, value)
		}
		return strconv.FormatUint(parsed, 10), nil
	case kindBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("field %q has invalid bool default %q", fieldName, value)
		}
		return strconv.FormatBool(parsed), nil
	default:
		return "", fmt.Errorf("field %q has unsupported type %q", fieldName, kind)
	}
}

func parseExtraKnownKeys(value string) []string {
	parts := strings.Split(value, ",")
	keys := make([]string, 0, len(parts))

	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}

	return keys
}

func mergeKnownKeys(fields []configField, extras []string) []string {
	seen := make(map[string]struct{}, len(fields)+len(extras))
	knownKeys := make([]string, 0, len(fields)+len(extras))

	for _, field := range fields {
		if _, ok := seen[field.Key]; ok {
			continue
		}
		seen[field.Key] = struct{}{}
		knownKeys = append(knownKeys, field.Key)
	}
	for _, key := range extras {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		knownKeys = append(knownKeys, key)
	}

	return knownKeys
}

func renderSource(
	packageName string,
	typeName string,
	specName string,
	fields []configField,
	knownKeys []string,
) ([]byte, error) {
	uint32Fields := make([]configField, 0)
	boolFields := make([]configField, 0)

	for _, field := range fields {
		switch field.Kind {
		case kindUint32:
			uint32Fields = append(uint32Fields, field)
		case kindBool:
			boolFields = append(boolFields, field)
		default:
			return nil, fmt.Errorf("unknown field kind %q", field.Kind)
		}
	}

	var buffer bytes.Buffer
	buffer.WriteString("// Code generated by go generate; DO NOT EDIT.\n\n")
	fmt.Fprintf(&buffer, "package %s\n\n", packageName)
	buffer.WriteString("import \"github.com/hrko/dprint-plugin-shfmt/dprint\"\n\n")

	fmt.Fprintf(&buffer, "var %s = dprint.ConfigResolverSpec[%s]{\n", specName, typeName)

	fmt.Fprintf(&buffer, "\tUInt32Fields: []dprint.UInt32ConfigFieldSpec[%s]{\n", typeName)
	for _, field := range uint32Fields {
		fmt.Fprintf(&buffer, "\t\t{\n")
		fmt.Fprintf(&buffer, "\t\t\tKey: %q,\n", field.Key)
		fmt.Fprintf(&buffer, "\t\t\tDefaultValue: %s,\n", field.DefaultValueLiteral)
		fmt.Fprintf(&buffer, "\t\t\tAllowGlobalOverride: %t,\n", field.AllowGlobalOverride)
		fmt.Fprintf(&buffer, "\t\t\tGet: func(config %s) uint32 {\n", typeName)
		fmt.Fprintf(&buffer, "\t\t\t\treturn config.%s\n", field.FieldName)
		fmt.Fprintf(&buffer, "\t\t\t},\n")
		fmt.Fprintf(&buffer, "\t\t\tSet: func(config *%s, value uint32) {\n", typeName)
		fmt.Fprintf(&buffer, "\t\t\t\tconfig.%s = value\n", field.FieldName)
		fmt.Fprintf(&buffer, "\t\t\t},\n")
		fmt.Fprintf(&buffer, "\t\t},\n")
	}
	buffer.WriteString("\t},\n")

	fmt.Fprintf(&buffer, "\tBoolFields: []dprint.BoolConfigFieldSpec[%s]{\n", typeName)
	for _, field := range boolFields {
		fmt.Fprintf(&buffer, "\t\t{\n")
		fmt.Fprintf(&buffer, "\t\t\tKey: %q,\n", field.Key)
		fmt.Fprintf(&buffer, "\t\t\tDefaultValue: %s,\n", field.DefaultValueLiteral)
		fmt.Fprintf(&buffer, "\t\t\tAllowGlobalOverride: %t,\n", field.AllowGlobalOverride)
		fmt.Fprintf(&buffer, "\t\t\tGet: func(config %s) bool {\n", typeName)
		fmt.Fprintf(&buffer, "\t\t\t\treturn config.%s\n", field.FieldName)
		fmt.Fprintf(&buffer, "\t\t\t},\n")
		fmt.Fprintf(&buffer, "\t\t\tSet: func(config *%s, value bool) {\n", typeName)
		fmt.Fprintf(&buffer, "\t\t\t\tconfig.%s = value\n", field.FieldName)
		fmt.Fprintf(&buffer, "\t\t\t},\n")
		fmt.Fprintf(&buffer, "\t\t},\n")
	}
	buffer.WriteString("\t},\n")

	buffer.WriteString("\tKnownKeys: []string{\n")
	for _, key := range knownKeys {
		fmt.Fprintf(&buffer, "\t\t%q,\n", key)
	}
	buffer.WriteString("\t},\n")

	buffer.WriteString("}\n")

	return buffer.Bytes(), nil
}

func exitWithError(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "gen-config-resolver: %v\n", err)
	os.Exit(1)
}
