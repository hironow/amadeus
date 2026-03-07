# Semgrep Rule / ADR Cross-Reference (CANONICAL — identical across all tools)

## shared-adr.yaml (13 rules)

| Rule ID | ADR | Description |
|---------|-----|-------------|
| adr0001-cobra-run-without-rune | phonewave 0001 | cobra RunE 必須 |
| adr0003-otel-set-tracer-provider-outside-init | phonewave 0003 | OTel TracerProvider 初期化制約 |
| adr0003-otel-set-meter-provider-outside-init | phonewave 0003 | OTel MeterProvider 初期化制約 |
| adr0003-otel-span-without-defer-end | phonewave 0003 | OTel span の defer End 必須 |
| adr0005-fsnotify-watcher-without-close | phonewave 0005 | fsnotify Watcher の defer Close 必須 |
| adr0005-string-concat-file-path | phonewave 0005 | filepath.Join 必須 (+ 結合禁止) |
| adr0005-mutex-lock-without-defer-unlock | phonewave 0005 | Mutex Lock の defer Unlock 必須 |
| adr0007-testcontainers-generic-without-terminate | phonewave 0007 | testcontainers Terminate 必須 |
| adr0007-mock-in-docker-test | phonewave 0007 | Docker テストでの mock 禁止 |
| adr0008-signal-notify-instead-of-notifycontext | phonewave 0008 | signal.NotifyContext 必須 |
| adr0008-execute-without-context | phonewave 0008 | Context 付き実行必須 |
| adr0009-os-getwd-in-cobra-cmd | phonewave 0009 | cobra cmd 内 os.Getwd 禁止 |
| d4-sql-open-without-defer-close | CLAUDE.md | sql.Open の defer Close 必須 |

## layers.yaml (9 rules)

| Rule ID | ADR | Description |
|---------|-----|-------------|
| layer-root-no-import-internal | phonewave S0007 | root → internal import 禁止 |
| layer-session-no-import-cmd | phonewave S0007 | session → cmd import 禁止 |
| layer-domain-no-import-upper | phonewave S0007 | domain → 上位層 import 禁止 |
| layer-eventsource-no-import-upper | phonewave S0007 | eventsource → 上位層 import 禁止 |
| layer-cmd-no-import-domain | phonewave S0007 | cmd → domain 直接 import 禁止 |
| s0008-cmd-no-import-eventsource | phonewave S0008 | cmd → eventsource 直接 import 禁止 |
| layer-usecase-no-import-cmd | phonewave S0015 | usecase → cmd import 禁止 |
| layer-usecase-no-import-eventsource | phonewave S0015 | usecase → eventsource import 禁止 |
| layer-session-no-import-usecase | phonewave S0015 | session → usecase import 禁止 |

## cobra.yaml (7 rules)

| Rule ID | ADR | Description |
|---------|-----|-------------|
| cobra-traverse-run-hooks-init | phonewave 0001 | EnableTraverseRunHooks は init() 内のみ |
| cobra-command-in-root | phonewave 0001 | cobra.Command は internal/cmd/ のみ |
| cobra-persistent-prerun-not-rune | phonewave 0001 | PersistentPreRunE 必須 |
| cobra-persistent-postrun-not-rune | phonewave 0001 | PersistentPostRunE 必須 |
| cobra-prerun-not-rune | phonewave 0001 | PreRunE 必須 |
| cobra-postrun-not-rune | phonewave 0001 | PostRunE 必須 |
| cobra-signal-stop-without-context | phonewave 0008 | signal.Stop goroutine leak 防止 |

## stdio.yaml (6 rules)

| Rule ID | ADR | Description |
|---------|-----|-------------|
| adr0002-no-fmt-print-in-session | phonewave 0002 | session 層の fmt.Print 禁止 |
| adr0002-no-fmt-print-in-eventsource | phonewave 0002 | eventsource 層の fmt.Print 禁止 |
| adr0002-no-fmt-print-in-root | phonewave 0002 | root パッケージの fmt.Print 禁止 |
| adr0002-no-os-stdout-in-internal | phonewave 0002 | internal/ の os.Stdout 禁止 |
| adr0002-no-os-stderr-in-internal | phonewave 0002 | internal/ の os.Stderr 禁止 |
| adr0002-no-os-stdin-in-internal | phonewave 0002 | internal/ の os.Stdin 禁止 |
