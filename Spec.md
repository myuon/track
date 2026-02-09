# Spec.md

## 1. Goal
- GitHub上のPR/デフォルトブランチpushで自動的にテストCIが実行され、失敗時に即座に検知できるようにする。
- 実装後はPRを作成し、`track gh` コマンドで関連PRを監視できる状態にする。

## 2. Non-Goals
- lint/typecheckの追加
- coverage uploadやバッジ表示
- 複数OS/複数Goバージョンのマトリクス化
- デプロイ用CI/CDの構築

## 3. Target Users
- `track` リポジトリの開発者
- GitHub Pull Requestで変更をレビュー/マージするメンテナー

## 4. Core User Flow
- 開発者がブランチで変更を作成してPushする。
- GitHub Actionsが `go test ./...` を実行する。
- PR画面でCI結果（pass/fail）を確認する。
- 開発者が `track gh link <issue_id> --pr <PR番号 or URL>` でissueとPRを紐付ける。
- 開発者が `track gh watch` を実行し、PRの状態変化（特にmerge）を追跡する。

## 5. Inputs & Outputs
- 入力
  - GitHubイベント（`pull_request`, `push`）
  - テストコードおよびGoソース
  - `track gh link` 実行時のissue ID / PR参照
- 出力
  - GitHub Actionsの実行ログと成功/失敗ステータス
  - `track gh status` / `track gh watch` の監視出力

## 6. Tech Stack
- 言語・ランタイム: Go
- フレームワーク: GitHub Actions workflow
- データベース: なし（既存リポジトリ管理範囲外）
- 主要ライブラリ/アクション: `actions/checkout`, `actions/setup-go`
- テストフレームワーク: Go標準 `testing`（`go test ./...`）

## 7. Rules & Constraints
- CI対象は最小構成として `go test ./...` のみ。
- トリガーは `pull_request` とデフォルトブランチへの `push`。
- 既存の開発フローを壊さないよう、追加は `.github/workflows` 配下に限定する。
- (仮) ブランチ保護設定はリポジトリ管理者側で行う（本実装の範囲外）。

## 8. Open Questions（必要な場合のみ）
- なし

## 9. Acceptance Criteria（最大10個）
- [ ] `.github/workflows` 配下にGoテストCI定義ファイルが追加されている。
- [ ] CIは `pull_request` イベントで起動する。
- [ ] CIはデフォルトブランチへの `push` で起動する。
- [ ] CIジョブ内で `go test ./...` が実行される。
- [ ] テスト失敗時にワークフローは失敗ステータスになる。
- [ ] テスト成功時にワークフローは成功ステータスになる。
- [ ] READMEまたは運用手順にPR作成後の監視フロー（`track gh`）が明記される。
- [ ] `track gh link` でissueとPRを紐付け可能である（既存機能の動作前提）。
- [ ] `track gh watch` 実行で紐付けPRの状態監視ができる（既存機能の動作前提）。

## 10. Verification Strategy
Agentの成果物が正しい方向に向かっているか、ゴールを達成したかを検証する方法。

- **進捗検証**: 実装中に正しい方向へ進んでいるかの確認方法
  - Workflow YAMLの構文確認と `go test ./...` のローカル実行で、CIが想定コマンドを実行可能かを都度確認する。
- **達成検証**: ゴールを達成したと言えるかの判断基準
  - PR上でGitHub Actionsの該当ジョブが実行され、成功/失敗が期待どおり表示されることを確認する。
- **漏れ検出**: 実装の漏れやサボりがないかの確認方法
  - 一時的に失敗テストを追加または既存失敗ケースを再現し、ワークフローがredになることを確認する。

## 11. Test Plan
- Scenario 1
  - Given テストが全て通る状態
  - When PRを作成する
  - Then GitHub Actionsのテストジョブが成功する
- Scenario 2
  - Given 失敗するテストが含まれる状態
  - When PRを更新するpushを行う
  - Then GitHub Actionsのテストジョブが失敗する
- Scenario 3
  - Given issueとPRを `track gh link` で紐付け済み
  - When `track gh watch` を実行する
  - Then 紐付けPR状態の監視出力が得られる
