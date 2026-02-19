# Panic Isolation Result

## How to Run

```bash
investigation/isolation/run.sh
```

## Cases

- `case_no_maps`: minimal dprint wasm plugin without `maps.Copy` and without `mvdan.cc/sh/v3/syntax`.
- `case_maps_copy`: same minimal plugin, but uses `maps.Copy` in `get_resolved_config`.
- `case_import_syntax`: same minimal plugin as `case_no_maps`, plus only `_ "mvdan.cc/sh/v3/syntax"` import.
- `case_stash_835cd96`: `refs/stash@{2}` (`835cd96a3424d2e2b916c6b5e74ce8ef2680a51b`) の `main.go` をそのままケース化。
- `case_syntax_json_plugininfo`: `syntax` + `get_plugin_info` で `json.Marshal` のみ。
- `case_syntax_json_unmarshal_only`: `syntax` + `register_config` で `json.Unmarshal` のみ。
- `case_syntax_json_resolved_marshal_const`: `syntax` + `get_resolved_config` で固定 struct を `json.Marshal`。
- `case_syntax_store_literal_no_read`: `syntax` + `register_config` で map 書き込みのみ（`get_resolved_config` で map を読まない）。
- `case_syntax_store_literal_no_marshal`: `syntax` + map 書き込み/読み取り（`get_resolved_config` で map lookup、Marshal なし）。
- `case_syntax_store_literal`: `syntax` + map 書き込み/読み取り + `json.Marshal`。
- `case_syntax_slice_store`: `syntax` + slice ベースの config 書き込み/読み取り（map を使わない）。
- `case_syntax_json_register_config`: `syntax` + `register_config` で Unmarshal + map 書き込み/読み取り。
- `case_syntax_json_register_typed`: `syntax` + typed config で Unmarshal + map 書き込み/読み取り。
- `case_syntax_global_map_unused`: `syntax` + 未使用 global map（読み書きなし）。

## Observed Result

- Success:
  - `case_no_maps`
  - `case_maps_copy`
  - `case_stash_835cd96`
  - `case_syntax_json_plugininfo`
  - `case_syntax_json_unmarshal_only`
  - `case_syntax_json_resolved_marshal_const`
  - `case_syntax_store_literal_no_read`
  - `case_syntax_global_map_unused`
- Failed (`RuntimeError: unreachable`):
  - `case_import_syntax`
  - `case_syntax_store_literal_no_marshal`
  - `case_syntax_store_literal`
  - `case_syntax_json_register_config`
  - `case_syntax_json_register_typed`
- `case_syntax_slice_store` は成功（同じ lifecycle でも map を使わない場合）。

Logs are written to `.tmp/isolation/logs/*.log`.

## Conclusion

- `refs/stash@{2}` のコード（`case_stash_835cd96`）は現環境でも trap しない。
- よって「`mvdan.cc/sh/v3/syntax` を import しただけ」では再現しない。
- 再現に必要なのは、`syntax` をリンクした状態で `register_config`/`get_resolved_config` 相当の **config map の書き込み + 読み取り** を行うこと。
- `json.Marshal`/`json.Unmarshal` 自体は単体では必須条件ではない（単体ケースは成功）。
- 同等の処理を slice で置き換えると成功するため、問題は map 操作側に寄っている。

現行実装で該当する疑い箇所:

- `dprint/runtime.go:82` (`r.unresolvedConfig[id] = config`)
- `dprint/runtime.go:289` (`unresolvedConfig, ok := r.unresolvedConfig[configID]`)

つまり、Stash 版から現行版への変更点のうち、`dprint.Runtime` による config lifecycle（map state 管理）が trap の主因候補。
