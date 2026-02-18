# dprint-plugin-shfmt 詳細設計書 (Schema Version 4)

本ドキュメントは、Go製フォーマッター `shfmt` (`mvdan.cc/sh`) を dprint の Wasm プラグインとして統合するための技術仕様を定義する。

## 1. システム構成概要

| **項目** | **仕様** |
| --- | --- |
| **開発言語** | Go (TinyGo コンパイラ使用) |
| **ターゲット** | WebAssembly System Interface (`wasm32-wasi`) |
| **プラグインスキーマ** | dprint Schema Version 4 |
| **コアライブラリ** | `mvdan.cc/sh/v3` |
| **メモリモデル** | 線形メモリ上の共有バッファを用いたホスト/ゲスト通信 |

## 2. メモリ管理と通信プロトコル

ホスト (dprint CLI) とゲスト (Wasmプラグイン) 間のデータ交換は、ゲスト側のメモリ空間に確保された共有バイトバッファを介して行う。

### 2.1 低レベルエクスポート関数

以下の関数をWasmモジュールからエクスポートし、メモリアドレス操作を提供する。

- **`get_shared_bytes_ptr() -> u32`**

    - 現在の共有バッファ（バイトスライス）の先頭ポインタ（オフセット）を返す。
    - 実装詳細: `unsafe.Pointer` を使用してスライスの基礎配列のアドレスを取得する。
- **`clear_shared_bytes(size: u32) -> u32`**

    - 指定された `size` バイト分の領域を確保または再利用し、ゼロクリアする。
    - 戻り値: 確保されたバッファの先頭ポインタ。
    - **バッファプーリング戦略**: 要求サイズが現在の `cap` 内であれば再割り当てを行わず `len` のみを変更する。超過する場合は `make` で再確保する。

### 2.2 メモリアロケータ設定

- **アロケータ**: TinyGo標準のGCではなく、`nottinygc` 等のカスタムアロケータまたは `-gc=leaking` (短命プロセスの場合) を検討し、AST構築時のオーバーヘッドを低減する。

## 3. エクスポート関数仕様 (ライフサイクル)

### 3.1 初期化・メタデータ

- **`dprint_plugin_version_4() -> u32`**

    - 定数 `4` を返す (スキーマバージョンの宣言)。
- **`get_plugin_info() -> u32`**

    - 以下のJSONをシリアライズして共有バッファに書き込み、そのサイズを返す。

            { "name": "dprint-plugin-shfmt", "version": "x.y.z", "configKey": "shfmt", "fileExtensions": ["sh", "bash", "zsh", "ksh", "bats"], "helpUrl": "https://...", "configSchemaUrl": ""}
- **`get_license_text() -> u32`**

    - プラグインおよび `shfmt` (BSD-3-Clause) のライセンス全文を返す。

### 3.2 コンフィギュレーション管理

- **`register_config(config_id: u32)`**

    - 共有バッファ内のJSON設定データを読み取り、`config_id` に紐付けて内部状態に保存する。
    - グローバル設定 (`GlobalConfiguration`) とプラグイン固有設定をマージして保持する。
- **`release_config(config_id: u32)`**

    - 指定されたIDの設定データをメモリから解放する。
- **`get_config_diagnostics(config_id: u32) -> u32`**

    - 設定値の型不正や未知のプロパティを検証し、診断情報の配列をJSONで返す (エラーなしの場合は `[]`)。
- **`get_resolved_config(config_id: u32) -> u32`**

    - デフォルト値やグローバル設定が適用された最終的な設定オブジェクトをJSONで返す。

### 3.3 フォーマット実行フロー

1. **`set_file_path()`**

    - ホストからファイルパスを受け取り、内部変数に保持する。拡張子によるシェル方言 (Dialect) 推論に使用する。
2. **`set_override_config()`**

    - (オプション) ファイル固有の上書き設定を受け取る。
3. **`format(config_id: u32) -> u32`**

    - 共有バッファ内のソースコードを読み取り、フォーマットを実行する。
    - **戻り値**:

        - `0`: 変更なし (No Change)
        - `1`: 変更あり (Change)
        - `2`: エラー (Error)
4. **`get_formatted_text() -> u32`**

    - `format` が `1` を返した場合に呼び出される。整形後のコードを返す。
5. **`get_error_text() -> u32`**

    - `format` が `2` を返した場合に呼び出される。パースエラー等の詳細メッセージを返す。

## 4. shfmt統合と設定マッピング

### 4.1 処理パイプライン

1. **入力**: 共有バッファ -&gt; `string` 変換 (ゼロコピー推奨)。
2. **方言 (Variant) 決定**: ファイル拡張子またはシバン (`#!`) から `syntax.LangBash`, `syntax.LangPOSIX`, `syntax.LangMirBSDKorn` を自動判定。
3. **パース**: `syntax.NewParser()` および `parser.Parse()` でAST構築。

    - エッジケース対応: 連想配列インデックスのクオート漏れ、算術式 (`$((`) の競合等のエラーを捕捉。
4. **プリント**: `syntax.NewPrinter()` にオプションを適用し、バッファへ書き出し。
5. **差分判定**: 入力と出力をバイト比較。

### 4.2 設定マッピングテーブル

| **dprintプロパティ** | **型** | **デフォルト** | **shfmtオプション (syntax.PrinterOption)** | **備考** |
| --- | --- | --- | --- | --- |
| `indentWidth` | `u32` | `2` | `Indent(uint)` | グローバル設定優先。`useTabs` true時は無視(0) |
| `useTabs` | `bool` | `false` | `Indent(0)` | グローバル設定優先。trueならハードタブ使用 |
| `binaryNextLine` | `bool` | `false` | `BinaryNextLine(bool)` | ` |
| `switchCaseIndent` | `bool` | `false` | `SwitchCaseIndent(bool)` | `case` 本体をインデント |
| `spaceRedirects` | `bool` | `false` | `SpaceRedirects(bool)` | `>file` → `> file` |
| `keepPadding` | `bool` | `false` | `KeepPadding(bool)` | 列揃えのスペースを維持 |
| `funcNextLine` | `bool` | `false` | `FuncNextLine(bool)` | 関数の中括弧 `{` を次行へ配置 |
| `minify` | `bool` | `false` | `Minify(bool)` | (オプション) 最小化を行う場合 |

## 5. ビルド構成

TinyGoを使用し、Wasmバイナリサイズと実行効率を最適化する。

### 5.1 ビルドコマンド

    tinygo build \ -o plugin.wasm \ -target=wasi \ -scheduler=none \ -no-debug \ -gc=custom \ -tags="custommalloc" \ main.go

### 5.2 フラグ解説

- **`-target=wasi`**: OS非依存のWASIインターフェースを使用。`wasm_exec.js` 依存を排除。
- **`-scheduler=none`**: Goroutineスケジューラを無効化し、シングルスレッド実行に特化させることでバイナリサイズを削減。
- **`-no-debug`**: DWARFデバッグ情報を削除。
- **`-gc=custom`**: メモリ集約的なAST処理のため、コンパイラ標準のGCではなくパフォーマンス指向のアロケータを使用。