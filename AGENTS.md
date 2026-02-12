# AGENTS.md

## impl default settings

When running `$impl` in this repository, use the following defaults unless explicitly overridden by the user:

- Review method: self review
- Check method: auto detect from project files (`package.json`, `Makefile`, `pyproject.toml`, `Cargo.toml`, etc.)

## note

If check commands fail due to intentional WIP or red-phase tests, allow commit only with an explicit note in the commit message.

## 開発ワークフロー

### ステータス遷移

1. 基本遷移は `todo -> ready -> in_progress -> done` とする。
2. planning で実装着手可能と判断できた issue のみ `ready` にする。
3. 追加回答待ちがある issue は non-ready のまま維持する。

### planning フェーズ

1. `track show <id>` を起点に要件・制約・受け入れ条件を確認する。
2. issue body の `## Spec` を実装可能な内容に更新する。
3. 不明点が残る場合は issue body に `## Questions for user` を追記し、`assignee=user` にする。
4. 不明点がある issue があっても、その issue で止まらず planning 対象の残り issue を継続する。
5. readiness 判定:
   - 実装着手可能なら `status=ready` に更新する。
   - 追加回答待ちなら `status` は non-ready のままにする。

### 実装フェーズ

1. 実装開始前に専用 branch を作成する。
2. 実装は意味のある単位で進める。
3. 各単位で review/check を実行し、通過後に commit する。
4. check が意図的な WIP または赤テストで失敗している場合は、commit message に明示ノートを残す。
