# Spec.md

## 1. Goal
- ローカルファーストなCLIツール `track` で、Issueの作成・更新・検索・状態遷移・並び替え・入出力・フック自動化をオフラインでも実行できるようにする。
- 必要時のみローカルWeb UIを起動し、Issueの閲覧と軽微な編集を行えるようにする。

## 2. Non-Goals
- フル機能のScrum/Agile管理（ベロシティ、スプリント計画、バーンダウン等）は実装しない。
- 大規模組織向けの権限管理、監査ログ、SSOは実装しない。
- 依存関係グラフ、ガントチャート等の重いPM機能は実装しない。
- GitHub App / Webhookベースのサーバー連携は実装しない（MVPは `gh` CLI pollingのみ）。
- Cloudの高度な競合解決（event sourcing等）は実装しない。

## 3. Target Users
- 個人開発者（単一ユーザー運用）が、日々の開発タスクを端末中心で高速に管理する。
- 利用シーン: コーディング中にCLIでIssue操作し、必要な時だけUIを開いて一覧確認や軽微な編集を行う。

## 4. Core User Flow
- ユーザーが `track new "<title>"` でIssueを作成する。
- ユーザーが `track list` / `track show <id>` で対象Issueを確認する。
- ユーザーが `track set` / `track label` / `track next` で属性更新と次アクション明確化を行う。
- ユーザーが `track reorder` で手動優先順（グローバル1本）を調整する。
- ユーザーが `track done` / `track archive` でライフサイクルを進める。
- 状態遷移や更新時に該当hookイベントを発火し、登録済みコマンドを実行する。
- 必要に応じて `track export` / `track import` で他ツール連携や移行を行う。
- 必要時のみ `track ui --port <n> --open` でWeb UIを起動して閲覧・軽編集する。
- GitHub連携を使う場合、`track gh link` でPRを紐づけ、`track gh watch` でPR merge等を監視してIssue状態を更新する。

## 5. Inputs & Outputs
- 主な入力（ユーザー入力 / 外部データ）
- CLIコマンド引数・オプション（title, status, priority, due, labels, assignee, next_action等）。
- `track import` の入力ファイル（text/csv/jsonl）。
- `track gh watch` 実行時の `gh` CLI出力（PR状態、CI結果）。
- `track sync` 実行時のCloud APIレスポンス（将来MVP Cloud段階）。
- `~/.track/config.toml` の設定値。
- 主な出力（表示 / 保存 / 生成物）
- 端末表示（list/show/status/help/error）。
- `~/.track/track.db` へのIssue・フック・連携情報の永続化。
- `track export` の出力（text/csv/jsonl）。
- フック実行結果（終了コード、ログ）。
- `track ui` が提供するローカルHTTPレスポンス（一覧・詳細・軽編集フォーム）。

## 6. Tech Stack
- 言語・ランタイム
- Go 1.23+
- フレームワーク
- CLI: `cobra`
- Web UI/API: Go標準 `net/http` + `html/template`
- データベース（必要な場合）
- SQLite（`modernc.org/sqlite`）
- 主要ライブラリ
- 設定: `BurntSushi/toml`
- バリデーション: `go-playground/validator`
- 排他制御: `gofrs/flock`
- テストフレームワーク
- `go test`（unit/integration）

## 7. Rules & Constraints
- ローカルファーストを守り、すべてのCLI操作はローカルDBへ即時反映する。
- 永続化パスは `~/.track/` 固定とし、MVPでは `track.db` と `config.toml` を使用する。
- Issue IDは `I-000001` 形式のローカル連番とする。
- Statusは `todo | in_progress | done | archived` の4種のみ許可する。
- Priorityは `p0 | p1 | p2 | p3` の4段階のみ許可する。
- Due dateは `YYYY-MM-DD` 形式のみ受け付け、無効日付はエラーにする。
- `reorder` はグローバル単一キューとして管理し、status別キューは実装しない。
- Hooksは定義済みイベント（`issue.created`, `issue.updated`, `issue.status_changed`, `issue.completed`, `sync.completed`）のみ登録可能。
- Hook実行コマンドは明示的登録のみを実行し、シェル文字列はそのまま解釈しない（引数分離実行を基本にする）。
- `track gh watch` は `gh` CLI が利用可能な環境でのみ動作し、未導入時は明確なエラーを返す。
- オフライン時は失敗せずローカル操作を継続し、sync要求のみ保留キューに積む。

## 8. Open Questions（必要な場合のみ）
- Track Cloud API仕様（エンドポイント、認証方式、競合解決詳細）はMVP Cloud開始時に別紙で確定する。
- Hookのサンドボックス方針（実行ユーザー制限、ネットワーク制限）はMVPでは最小制約とし、運用フェーズで強化方針を確定する。

## 9. Acceptance Criteria（最大10個）
- `track new "A"` 実行後、`track list` に新規Issueが1件表示される。
- `track set <id> --status in_progress` 実行後、`track show <id>` のstatusが `in_progress` になる。
- `track done <id>` 実行後、`track show <id>` のstatusが `done` になる。
- `track archive <id>` 実行後、`track show <id>` のstatusが `archived` になる。
- `track label add <id> ready` 実行後、`track list --label ready` で当該Issueが表示される。
- `track next <id> "PRを作る"` 実行後、`track show <id>` に `next_action` が表示される。
- `track reorder <id2> --before <id1>` 実行後、`track list --sort priority` とは別に手動順表示で `<id2>` が `<id1>` より前になる。
- `track export --format csv` の出力を `track import --format csv` に入力すると、件数整合が取れる。
- `track hook add issue.completed --run "echo ok"` 登録後、対象Issueを `done` にするとhookが1回実行される。
- `track gh watch --repo owner/name` 実行中に紐づけ済みPRがmergeされると、対応Issueのstatusが `done` になる。

## 10. Verification Strategy
Agentの成果物が正しい方向に向かっているか、ゴールを達成したかを検証する方法。

- **進捗検証**: 実装中に正しい方向へ進んでいるかの確認方法
  - 各コマンド群（Issue lifecycle / Hooks / Import-Export / GitHub / UI）ごとに、最小デモ手順を実行してCLI出力を確認する。
  - 主要ロジック（status遷移、ID採番、filter/sort、hook発火、import/export）に対し `go test` を段階的に追加し、各機能完了時にテストを通す。
- **達成検証**: ゴールを達成したと言えるかの判断基準
  - `Acceptance Criteria` 10項目をチェックリスト化し、全項目が `Yes` になった時点でMVP達成とする。
  - 新規環境で `track --help` から基本フロー（new/list/set/done/export/import）を再現し、オフラインでも破綻しないことを確認する。
- **漏れ検出**: 実装の漏れやサボりがないかの確認方法
  - 仕様上のコマンド一覧と実装済みコマンド一覧を突合し、未実装を0件にする。
  - DBスキーマ項目と `Issue` フィールド定義を突合し、欠落カラムを0件にする。
  - 失敗系テスト（不正日付、不正status、存在しないID、`gh` 未導入）を最低1件ずつ用意する。

## 11. Test Plan
- e2e シナリオ1（Issue基本フロー）
  - Given: 初期化済みローカル環境（空DB）
  - When: `new -> set status -> next -> done -> show` を順に実行する
  - Then: 対象Issueが期待どおり更新され、最終statusが `done` で表示される
- e2e シナリオ2（Import/Export整合）
  - Given: 複数Issueが登録済みのローカルDB
  - When: `export --format csv` 後に別DBへ `import --format csv` する
  - Then: 件数と主要フィールド（title/status/priority/labels）が一致する
- e2e シナリオ3（GitHub watch連携）
  - Given: PRリンク済みIssueと `gh` 利用可能環境
  - When: `track gh watch` 実行中に対象PRがmergeされる
  - Then: 対応Issueが自動で `done` に遷移し、status変更イベントが記録される
