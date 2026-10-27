# No.11–14 運営審査・通知・パスワード再設定

## 適用範囲と既存仕様

一般ユーザーの登録・ログイン・Cookie `auma_session` は継続する。
運営は `admin_users` / `admin_sessions` / Cookie `auma_info_session` を使用する。
`users.role=0` は会社の管理者であり、運営権限として扱わない。
ブランド・ショップの `approval` / `status` によるログイン禁止や会社全体への制限は追加しない。
審査は `submitted_at IS NOT NULL AND approval=0` の対象だけに行う。
`approval` を1または2に変更し、`status` / `submitted_at` / パスワードは変更しない。

## Migration

追加: `migrations/0014_create_review_auth_notifications.sql`。
追加テーブル: `admin_users`, `admin_sessions`, `notification_events`,
`password_reset_tokens`, `review_decisions`, `security_request_limits`。
既存テーブル・Migrationは変更しない。既存データの書換えはない。
`review_decisions` は審査担当者・対象会社・判定の監査記録、
`security_request_limits` は新規公開認証APIのレート制限用。

本作業では `SELECT current_database()` で `auma_test` を確認し、テストDBだけに適用。
開発・本番DBへの適用は別の管理作業とする。

## 設定

APIと通知ワーカーに以下の環境変数を設定する（Composeにも引き継ぐ）。

- `ADMIN_NOTIFICATION_EMAILS`: 運営通知先。カンマ区切り、少なくとも1件。
- `APP_URL`: HTTPSのフロントエンドURL。運営通知のリンクは `/info/brands/:id` または `/info/shops/:id`。
- `PASSWORD_RESET_URL`: HTTPSの再設定画面URL。`token` クエリを付けてメールに記載する。
- `NOTIFICATION_ENCRYPTION_KEY`: 暗号学的乱数32バイトのBase64。例: 管理環境で `openssl rand -base64 32` により生成し、秘密管理に保存する。固定の初期値はない。
- 既存 `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_ADDRESS`。
- `NOTIFICATION_SMTP_ALLOW_PLAINTEXT`: 通常は `false`。ローカルのメール捕捉環境でのみ `true` を指定できる。認証情報を平文接続で送ることは許可しない。

API・ワーカーは必須通知設定を起動時に検証する。
キーは再設定メールの保存本文をAES-256-GCMで暗号化するために使用する。
未送信イベントがある間は旧キーを失わないこと。変更すると復号できないイベントは
`failed/payload_unavailable` となり、キーの復旧が必要になる。

運営Cookieは常に `Secure; HttpOnly; SameSite=Strict; Path=/api/v1/info`。
ブラウザーからの運営利用にはHTTPSが必要。HTTPSを終端するプロキシを使う場合も
公開側のHostを保持し、Cookieやトークンのログ出力を無効にする。
新規APIはCORS許可を追加しない。運営の書込みはOrigin/Fetch Metadataも検証する。
パスワード再設定画面は本変更の対象外。画面側では `Referrer-Policy: no-referrer` を設定し、
トークン付きURLをアクセス解析・外部リソースに送らず、読取り後にURLから除去する。

## 初期運営アカウント作成

公開の運営登録APIはない。DBに接続できる管理環境でのみ実行する。
秘密管理から十分なランダム性のある12～72バイトのパスワードを、所有者のみが読める
通常ファイル（例: mode 0600）として用意する。シンボリックリンクは受け付けない。
パスワードをCLI引数・履歴・リポジトリに書かない。

```bash
cd src/backend
go run ./cmd/create-admin --email operator@example.com --password-file /secure/admin-password
```

接続には既存の `DB_*` 設定を用いる。対象DBを管理者が事前確認すること。
コマンドはアカウントを新規作成し、既存アカウントのパスワードを上書きしない。
作成後は秘密ファイルを組織の秘密管理手順に従って処理する。本作業では運営初期アカウントを作成していない。

## API

以下はすべて `/api/v1` 配下。新規JSON入力は未知のキー・余分なJSON・16KiB超を拒否する。
通常レスポンスにトークン・パスワード・パスワードハッシュは含めない。

| Method | URL | 入力 / 応答 |
|---|---|---|
| POST | `/info/login` | `{"email":"...","password":"..."}` → 200 `{"admin":{"id":1,"email":"..."}}` と運営Cookie |
| POST | `/info/logout` | 運営認証必須。200 `{"success":true}`、DBのセッションを削除しCookie失効 |
| GET | `/info/me` | 運営認証必須。200 `{"admin":{"id":1,"email":"..."}}` |
| GET | `/info/brands`, `/info/shops` | 運営認証必須。申請済み・審査待ちを `{"items":[...]}` で返す |
| GET | `/info/brands/:id`, `/info/shops/:id` | 運営認証必須。申請済み対象の `entity`, `company`, `categories`, `images`, `price_screening` |
| PATCH | `/info/brands/:id/review`, `/info/shops/:id/review` | 運営認証必須。`{"company_id":123,"approval":1}` または2 → 200 `{"id":456,"company_id":123,"approval":1}` |
| GET | `/info/notification-events` | 運営認証必須。`{"items":[{"id":1,"event_key":"...","recipient":"...","subject":"...","state":"pending","attempts":0,"last_error":null,"created_at":"...","sent_at":null}]}` |
| POST | `/password-reset/request` | 未ログイン可。`{"email":"..."}` → 202。存在しないメールでも同じ本文 |
| POST | `/password-reset/confirm` | 未ログイン可。`{"token":"...","password":"..."}` → 200 `{"success":true}` |

一覧・イベントはID降順、最大50件。次ページは最後のIDを `?before_id=...` で指定する。
審査詳細は審査済みでも取得できる。未申請・対象なし・会社不一致は404、
未申請への審査操作・審査済みへの再操作は409、不正なID/判定は400。
一般Cookieだけで運営API、運営Cookieだけで一般APIを呼んだ場合は401。

運営ログインはメールごと5回/分、接続元IPごと30回/分。
再設定要求はメールごと3回/分、IPごと30回/分、再設定確定はIPごと30回/分。
超過は429。複数インスタンスでも制限を共有する。
`TRUSTED_PROXY_CIDRS` が空なら転送ヘッダーを無視し、ソケットのRemoteAddrを使用する。
明示設定したプロキシからの接続だけ、X-Forwarded-Forを右から検証し、最初の未信頼ホップをクライアントとする。
X-Real-IPは使用しない。ローカルNginxは外部から来たX-Forwarded-Forを上書きする。

## メール照合とイベント処理

Laravelの `RegisterMail`, `RegisterApprovalMail`, `ApprovalMail`, `LoginApprovalMail`,
`PasswordResetMail`, `CustomResetPassword` と各 `resources/views/emails/` を照合した。
申請完了・運営通知・承認／非承認の主文、件名、署名を継承する。
申請通知は実際の申請者、運営通知は環境設定の宛先、審査結果は対象会社の `role=0` の管理者宛て。
複数の会社管理者がいる場合は、Laravelの `first()` に合わせ1名（ID最小）に通知する。
運営通知先と申請者が同じメールでも、役割ごとの別通知として扱う。

意図した差異:

- `users.id=1` への固定送信を廃止。
- 承認時に既存パスワードを上書きせず、平文パスワードを送らない。
- 承認通知は登録時パスワードでのログインを案内する。
- 再設定は未ログインでも可能、期限60分、平文トークンをDBに保存しない。
- 通知のリンクは新運営画面・新再設定画面のURLを使用する。
- Laravelの審査トランザクション内SMTP送信を、永続イベントと別ワーカーに置き換える。

APIはSMTPに接続せず、状態更新と同じトランザクションでイベントを保存する。
イベント保存のDBエラー時は申請／審査もRollbackする。
コミット済みの審査状態は、後続の配送失敗では巻き戻らない。
同一申請・同一審査の同じ宛先へのイベントは一意制約で重複作成を防止する。
審査行ロックと監査記録の一意制約で二重審査も防止する。

別プロセスとして定期実行する。例えば1分ごとの管理スケジューラーに登録する。

```bash
cd src/backend
go run ./cmd/notification-worker --limit 50
```

| state | 意味と処理 |
|---|---|
| pending | 未送信。ワーカー対象 |
| sending | ワーカーが確保済み。SMTP処理中 |
| sent | SMTP DATAの受領成功を確認。自動再送しない |
| failed | 未受領が明確な失敗、または復号失敗。5分後から再試行対象 |
| unknown | DATA後の切断・不明なエラー・確保後5分以上の中断。自動再送しない |
| cancelled | 再設定トークン失効／期限切れ等で不要。自動再送しない |

複数ワーカーは `FOR UPDATE SKIP LOCKED` により同じイベントを確保しない。
SMTPのタイムアウトは30秒。ワーカーが送信後のDB更新前に停止した場合も再送せず、
次のワーカー実行で5分超の `sending` を `unknown` にする。
`last_error` は固定の安全なコードであり、例外本文・トークンを含めない。
SMTP Message-IDは `<auma-notification-イベントID@送信元ドメイン>`。
配送結果不明の場合はイベントID・宛先・日時とSMTP配送記録を照合する。
この変更には `unknown` を強制再送するAPIは設けていない。
通常SMTPで「配送結果不明でも必ず一度だけ届ける」ことは保証しない。

再設定メールの本文は暗号化して保存し、送信済み／結果不明／取消では本文を消去する。
再発行時は以前のトークンを失効し、未送信の旧イベントを取消する。
すでに送信中の旧メールが届くことはあり得るが、そのトークンは利用できない。
パスワード確定時はトークン消費・bcrypt保存・旧一般セッション削除を同一トランザクションで行う。
運営セッションは変更しない。

## 検証

テストは `TEST_DB_NAME=auma_test` と実接続の `current_database()` を確認して実行する。
本実装のHTTP統合テスト・ワーカーテストはテスト用Mailerのみを利用し、SMTPへ送らない。
以下の通知用設定はテスト専用値であり、実運用に使用しない。

```bash
docker compose run --rm --no-deps \
  -e DB_HOST=db_test -e DB_NAME=auma_test -e DB_USER=auma -e DB_PASSWORD=auma_test_password \
  -e ADMIN_NOTIFICATION_EMAILS=operation@example.com -e APP_URL=https://auma.example \
  -e PASSWORD_RESET_URL=https://auma.example/password-reset \
  -e NOTIFICATION_ENCRYPTION_KEY=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= \
  backend go test ./... -count=1
```

APIのURLは追加のみ。既存の一般ユーザー向け登録・ログイン・申請レスポンスは維持する。
運営画面・再設定画面のフロントエンドと、実SMTP配送の運用接続確認はこのバックエンド実装の対象外。

## 初回実装の新規・変更ファイル（F1〜F4修正前）

計26件。既存の `cookies.txt`, `shop-cookies.txt`, `memo.txt` は含めない。

- `docker-compose.yml`
- `src/backend/cmd/create-admin/main.go`
- `src/backend/cmd/notification-worker/main.go`
- `src/backend/controllers/info/controller.go`
- `src/backend/controllers/passwordreset/controller.go`
- `src/backend/docs/review-auth-notifications.md`
- `src/backend/internal/adminauth/filter.go`
- `src/backend/internal/adminauth/origin.go`
- `src/backend/internal/adminauth/repository.go`
- `src/backend/internal/adminauth/service.go`
- `src/backend/internal/httpinput/input.go`
- `src/backend/internal/notification/repository.go`
- `src/backend/internal/notification/service.go`
- `src/backend/internal/notification/smtp.go`
- `src/backend/internal/passwordreset/repository.go`
- `src/backend/internal/passwordreset/service.go`
- `src/backend/internal/registrationsubmission/repository.go`
- `src/backend/internal/review/repository.go`
- `src/backend/internal/review/service.go`
- `src/backend/main.go`
- `src/backend/migrations/0014_create_review_auth_notifications.sql`
- `src/backend/routers/router.go`
- `src/backend/tests/auth/main_test.go`
- `src/backend/tests/auth/review_workflow_test.go`
- `src/backend/tests/brand/notification_test.go`
- `src/backend/tests/shop/notification_test.go`

## F1〜F4のコミット前修正

公開APIのURL・JSON・レート制限閾値・審査業務は変更していない。Migration追加・適用はない。

- F1: 一般ユーザーのセッション作成時にユーザー行をFOR UPDATEでロックし、bcrypt認証に用いたハッシュと現在値を再照合する。同じトランザクション内でセッションを保存する。再設定が先なら旧ハッシュの発行を401として拒否し、ログインが先なら再設定側の既存セッション失効で削除する。再設定側のユーザー→トークン／セッションというロック順序を維持。
- F2: SMTP DATA本文のWrite失敗・短い書込みは接続を閉じ、DATA WriterのCloseを呼ばずErrNotAcceptedとして再送対象にする。終端送信後の応答喪失はunknownのまま自動再送しない。テストはnet.Pipeのテスト用SMTP接続を使い、外部へメール送信しない。
- F3: 信頼プロキシを環境設定で限定するIPリゾルバーを追加。運営ログイン・再設定要求・再設定確定で共通使用する。未設定・未信頼の接続から来た転送ヘッダーは無視する。
- F4: Rollbackテストの関数／トリガー名にbrand/shopと96ビットの乱数を含め、各テストのCleanupはその名前だけを削除する。両テストを並列実行対象にした。

### ローカルDocker/Nginx

`TRUSTED_PROXY_CIDRS` をバックエンド環境に追加した。カンマ区切りのIPまたはCIDRを指定する。
初期値は空であり、Dockerのプライベートネットワーク全体を自動的に信用しない。
不正な指定と全アドレスを信用する `0.0.0.0/0` / `::/0` は起動時に拒否する。

現在のNginxコンテナのIPだけを確認する例:

```bash
docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{println}}{{end}}' auma_dev_nginx
```

確認したIPだけを `TRUSTED_PROXY_CIDRS` に明示し、バックエンドを再起動して反映する。
コンテナ再作成等でIPが変われば再確認が必要。ネットワーク全体の許可で代用しない。
ブラウザーからNginxの `/api/` を経由する構成では、この設定により実クライアントごとの枠になる。
Viteの5173ポートから直接APIプロキシを利用する場合は接続元が別コンテナになるため、
その経路を利用する場合だけ、信頼するViteプロキシのIPも個別に確認する。
ローカルNginxはエッジとして `X-Forwarded-For $remote_addr` を設定し、外部からの偽装チェーンを破棄する。

本番のHTTP経路・プロキシIP/CIDRは未確定。本番値は推測せず未設定とする。
デプロイ時に、Go直前のプロキシと各ホップのヘッダー上書き／追記の設定を確認し、
信頼範囲を最小限に設定する。ALB等が前段にある配置へ、このローカルエッジ設定をそのまま転用しない。

### 回帰テストと列挙耐性の測定

- ログイン／再設定の両コミット順序と、PostgreSQLの実際のユーザー行ロック待ち後のハッシュ再照合。
- SMTP本文書込み失敗、DATA受領応答喪失、正常受領。
- 信頼プロキシ経由の別クライアント、同一IP、未信頼接続の偽装、偽装された左側XFFホップ、IPv6、未設定、不正設定。
- 運営ログイン・再設定要求・再設定確定について、既存の30回/分のIP制限とクライアント間分離。
- ブランド／ショップの並列Rollbackを各3回実行。

列挙耐性の時間差測定は `AUMA_MEASURE_RESET_TIMING=1` を設定し、
`go test ./tests/auth -run '^TestPasswordResetRequestTiming$' -count=1 -v` で実施する。
通常の回帰実行ではスキップする。測定に合否閾値や固定Sleepは設けない。

今回のローカルauma_testでのサービス／DB処理測定（各100件、順序をランダム化、事前ウォームアップあり）:

| 対象 | 平均 | 中央値 | p95 |
|---|---:|---:|---:|
| 存在するメール | 0.642 ms | 0.619 ms | 0.948 ms |
| 存在しないメール | 0.105 ms | 0.106 ms | 0.122 ms |

存在する側が100/100組で遅かった。測定ではメール単位レート制限を経由せずサービスを直接呼び、
DB処理差を分離している。HTTP／TLS／本番ネットワークを含む測定ではない。
外部からの列挙成功は未検証だが、約0.5 msの系統的な差があるため、応答本文の統一だけで
時間差による列挙も防げたとは判断しない。レート制限は測定を難しくするが、完全な対策ではない。
必要な対策候補は、要求受付を非同期化してアカウント検索・トークン発行を応答後に分離すること。
今回は測定・報告のみとし、APIの受付処理や業務フロー変更、固定Sleep追加は行わない。

### F1〜F4修正の変更ファイル

- `docker-compose.yml`
- `docker/nginx/default.conf`
- `src/backend/internal/auth/repository.go`
- `src/backend/internal/auth/service.go`
- `src/backend/internal/notification/smtp.go`
- `src/backend/internal/notification/smtp_test.go`
- `src/backend/internal/clientip/resolver.go`
- `src/backend/internal/clientip/resolver_test.go`
- `src/backend/controllers/info/controller.go`
- `src/backend/controllers/passwordreset/controller.go`
- `src/backend/routers/router.go`
- `src/backend/tests/auth/login_reset_race_test.go`
- `src/backend/tests/auth/proxy_rate_limit_test.go`
- `src/backend/tests/auth/reset_timing_test.go`
- `src/backend/tests/brand/notification_test.go`
- `src/backend/tests/shop/notification_test.go`
- `src/backend/docs/review-auth-notifications.md`
