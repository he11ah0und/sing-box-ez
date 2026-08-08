# Wails-фронтенд: архитектурный контракт и найденные проблемы

Рабочий документ для обсуждения. Фиксирует принцип взаимодействия фронтенда
и бэкенда, найденные отклонения от него и план приведения в порядок.
Ветка: `wails-migration`. Эталон поведения: старый Gio GUI (ветка `testing`).

## Принцип

Фронтенд — это только интерфейс:

- **рендерит состояние**, которое приходит из Go событиями;
- **отправляет намерения** пользователя (клик/ввод → вызов биндинга);
- **не содержит** таймеров, буферов-дублей, оркестрации вызовов
  и бизнес-логики.

Go — источник истины:

- владеет всеми циклами/таймерами (поллинг API, фоновые проверки);
- выполняет side-эффекты (применение темы/языка при сохранении настроек);
- пушит состояние событиями (`status:changed`, `configs:changed`,
  `traffic:updated`, `log:app` и т.д.);
- агрегирует payload'ы (фронт не должен делать N вызовов для одного экрана).

Поток данных односторонний: события Go → фронт; намерения фронт → биндинги Go;
результат — снова событиями.

## Модель локализации (актуально)

- Статические ключи объявляются один раз в `<script>` компонента как
  ячейки: `const commonSave = useLocale('common.save')`, в разметке —
  `{$commonSave}`. Ячейки мемоизированы (`locale.ts`), регистрация ключа
  на бэке происходит ровно один раз за сессию.
- `tValue($locale, key)` — только для динамических ключей
  (`` `main.api.mode_${m}` ``, `tab.key` из реестра страниц);
  `tValues($locale, [dyn, 'fallback.chain'])` — для цепочек с
  динамическим первым ключом.
- `getLocaleString(key)` — нереактивное чтение в обработчиках
  (тосты, сборка строк в логике).
- Английские фолбэки в коде запрещены: отсутствующее значение
  рендерится как сам ключ; источник истины —
  `internal/app/locales/*.yaml`, хардкод ловит `make i18n-hardcode`.
- Обновления приходят из Go: `locale:changed` / `locale:keys_changed`
  (`SetLanguage`, `LocaleReady`, ответ `RegisterLocaleKeys`).

## Модель тостов (актуально)

- Тосты приходят из Go событием `toast {level, text, description?}`;
  текст уже локализован (localengine, fmt-подстановки в Go).
  Фронт только отображает (`bridge.ts` → sonner).
- Биндинг, выполняющий действие пользователя, сам эмитит тост об
  успехе/ошибке (`toastT`/`toastErr` в `internal/gui/wails/toast.go`);
  обработчик на фронте — пустой `catch`.
- Clipboard через Go: `CopyLogs(source)`, `CopyValidationReport(name)` —
  форматирование текста, копирование и тост в одном месте.
- Исключение: ошибки пассивных загрузок (`Get*` при маунте страницы)
  пока показываются фронтом сырым текстом — при недоступном бэкенде
  локализовать нечего.
- Inline-ошибки форм (`configs.nameRequired`) — UI-логика формы,
  читаются из ячеек (`$cell`), на бэк не тащим.

## Найденные отклонения (фронт делает работу Go)

| # | Место | Что не так | Целевое состояние |
|---|-------|-----------|-------------------|
| 1 | `SettingsPage.svelte:82-94` | После `SaveSettings` фронт сам вызывает `GetTheme` + `SetLanguage` и применяет | `SaveSettings` в Go применяет язык/тему и эмитит `settings:changed`/`theme:changed`/`locale:changed` |
| 2 | `MainPage.svelte:100` | `setInterval(pollAPI, 2000)` — фронт поллит группы/ноды/соединения | Go-поллер + событие `api:changed {status, mode, groups, connections}` (как `refreshLoop` в Gio) |
| 3 | `bridge.ts` (`maxPoints = 60`) | История трафика копится во фронте хардкодом; настройка `core.traffic_graph_history` (30/60/120/300) есть в Go и выведена в UI, но ни на что не влияет | История собирается в Go-поллере, приходит в `traffic:updated`; настройка работает |
| 4 | `appState.ts` (`slice(-1000)`) | Буфер логов дублируется: Go уже хранит логи (`GetAppLogs`/`GetCoreLogs`) и эмитит строчки | `log.limit` применяется в Go; фронт: снапшот при маунте + доклейка событий |
| 5 | `StartupPage.svelte` | Фронт сам решает startup-флоу (`show=false` локально) | Решение в Go (событие с опциями) или выпилить целиком — service/remote режимы в Wails не используются |
| 6 | `ConfigsPage.svelte` | N+1: `IsConfigHashMismatch` дёргается на каждый конфиг | Флаг вшит в payload `GetConfigs`/`configs:changed` |
| 7 | `App.svelte:35` | `setTimeout(signalLocaleReady, 0)` — хак тайминга готовности бэкенда | Go эмитит `theme:changed`/`locale:changed` сам, когда готов |

Логи (п.4) — по договорённости: логи только приходят от бэка событиями;
фронт ничего своего не накапливает сверх снапшота.

## Состояние конфигов: что было в Gio и чего не хватает

### Было (testing: `internal/gui/gio/pages/configs_page.go`)

- **Цвет карточки по кешу**: `CardUncached` / `CardUncachedNoAutoUpdate` —
  видно, что конфиг ещё не скачан/не создан (кеш отсутствует).
- **Бейдж стиля** (`configStyle` → `inboundstyle.Detect`): `client` /
  `server` / `undefined`, цвет Success/Warning/Error, суффикс
  `not_recommended` для не-client стилей.
- **Бейдж типа**: `local` / remote; **источник**: `user` / плагин (`parent`).
- **Бейдж `modified`** — hash mismatch (warning-цвет).
- **Мета-строка**: `last_update`, `next_update` (`rec.NextUpdate()`),
  источник.
- Диалог добавления: тип remote/local (radio), имя, URL, период
  (дефолт из `updates.default_interval_hours`), auto-update.

### Сейчас шлёт бэкенд (`ConfigRecord` payload)

`name, url, type, update_interval_hours, last_update, parent, auto_update,
hash, fallback_type` — т.е. тип, источник, хэш и fallback_type уже есть,
фронт их просто не рендерит полностью.

### Чего не хватает (добавить в payload на стороне Go)

| Поле | Источник в Go | Зачем |
|------|---------------|-------|
| `cached` | `HasCachedConfig(name)` | цвет/бейдж «не скачан» |
| `style` | `DetectConfigStyle(name)` | бейдж client/server/undefined |
| `hash_mismatch` | `IsConfigHashMismatch(name)` | бейдж modified (убирает N+1, п.6) |
| `next_update` | `rec.NextUpdate()` | мета-строка (не считать на фронте) |

Все четыре вычисляются в Go при формировании `GetConfigs`/`configs:changed`.

## Открытые решения

1. **Settings: DTO vs KV.** Сейчас рукописная структура `Settings`
   (Get/Save маппинг в двух местах). KV-вариант (`GetSettings() map`,
   `SetSetting(path, value)`) логичнее для «тупого интерфейса», но теряет
   TS-типы. Третий путь — кодоген структуры/TS из sheet-схемы позже.
2. **Restart после ResetData.** В Gio был re-exec (`syscall.Exec`,
   `restart_unix.go`/`restart_windows.go`), умер с Gio. Сейчас ResetData
   завершает приложение. Портировать re-exec (~10 строк unix + windows)?
3. **StartupPage и режимы embed/service/remote.** В Gio был стартовый
   диалог выбора режима (embed/service/remote, TCP-адрес, «запомнить»,
   старт/стоп системного сервиса — `testing:internal/gui/gio/app.go:287,447,520,630-665`)
   и те же настройки в Settings → System (`remote.last_connection_mode`,
   `remote.remember_connection_mode`). В Wails отсутствует целиком:
   `StartupPage.svelte` — мёртвый стаб (`appState.startup.show` всегда
   `false`). Вместо remote-режима появился свой WebSocket IPC
   (`internal/gui/wails/websocket.go`) — но UI выбора режима он не заменяет.
   Решить: выпилить страницу и состояние или восстанавливать режимы?
4. **Плагины.** Движок/менеджер/CLI есть, в GUI нет (в Gio был стаб).
   Отложено до стабилизации остального.

## Детальный аудит Gio ↔ Wails (сверх состояния конфигов)

Критичное:

- **Страница Core недостижима.** Зарегистрирована с `nav: false`
  (`frontend/src/lib/pages/index.ts:54`), ни одна кнопка на неё не ведёт,
  `MenuPage` фильтрует `nav && !bottomNav` → скачать/обновить ядро из UI
  невозможно. В Gio вкладка Core жила внутри Settings
  (`testing:.../settings_page.go:544`).
- **Стартовые уведомления об обновлениях молчат.** Бэкенд эмитит
  `core:update_available` (`internal/gui/wails/about.go:181-184`), но
  слушателя во фронте нет; `selfupdate:available` виден только на About.
  В Gio при старте были диалоги с current/latest, датой релиза и
  markdown-ченджлогом (`testing:.../app.go:876,995`).
- **Мёртвые настройки.** `show_logs` сохраняется, но нигде не читается
  (в Gio динамически убирал страницу логов из навигации,
  `testing:.../app.go:165`); `log.level` вообще не портирован (в Gio была
  настройка + фильтр `GetLogLinesAtLeast`, `testing:.../log_page.go:174-181`);
  `traffic_graph_history` — см. отклонение 3.

Main-страница:

- `APIStatus.Uptime` приходит, но не показывается (в Gio шапка:
  `version + uptime`, `testing:.../main_page.go:440`).
- Сводка URL-test: биндинг `URLTestResult` отдаёт `Average`/`Count`
  (`internal/gui/wails/api.go:296-326`), фронт берёт только `results`
  (`MainPage.svelte:212-213`); в Gio было «avg X ms по N узлам».
- Click-to-copy в деталях соединения (Gio: `main_page.go:1185-1215`) — нет.
- Детект «ядро умерло до готовности API» со снятием спиннера
  (Gio: `main_page.go:1440-1452`) — нет.

Конфиги — формы:

- `auto_update` захардкожен `true` (`ConfigFormModal.svelte:78`); в Gio был
  чекбокс в add/edit-диалоге.
- Поле URL показывается и шлётся для local-типа; в Gio скрывалось и
  уходило `url=""` (`testing:.../configs_page.go:379-384`).
- Тип конфига редактируется в edit-диалоге; в Gio read-only
  (`configs_page.go:437-451`).
- Дефолт периода захардкожен 24 (`ConfigFormModal.svelte:41`); в Gio —
  `updates.default_interval_hours` из настроек.

Core (Gio `core_page.go`), чего нет:

- Карточка API info (type/address/secret + Copy); биндинг `APIInfo` не
  содержит `Secret` (`internal/gui/wails/api.go:13-19`).
- UpdateCheck-виджет: кнопка с текущей версией, детали в диалоге,
  состояния up-to-date/dev-build (`testing:.../widgets/update_check.go`).
- Подсветка блока обновления при missing core (мигание 3 сек).

Реакции на ошибки запуска:

- `OnCoreMissing`: Gio навигировал в Settings→Core и подсвечивал блок;
  Wails — notify + модалка без навигации.
- `OnConfigMissing`: Gio открывал страницу конфигов и сразу диалог
  добавления; Wails — только notify + модалка.

Self-update install:

- Confirm-диалог без ченджлога/даты, хотя `SelfUpdateInfo.Body`/
  `LatestDate` доступны (`about.go:91-102`); в Gio details до установки и
  «Update complete. Please restart.» после. Markdown в диалогах Wails не
  рендерится (plain text).

Окно/трей:

- «Minimize» из трея делает `win.Hide()` (окно исчезает,
  `internal/gui/wails/app.go:606`); в Gio был `system.ActionMinimize`.
- Заголовок окна хардкод «sing-box-ez» (`app.go:588`); в Gio —
  локализованный `T("app","title")`.

Прочее (осознанные различия, не требуют действий): трей общий
(`internal/gui/tray/tray.go`); Update all — спиннер + toast вместо
пер-конфиг прогресс-диалога; ResetData — выход вместо re-exec (решение 2);
desktop notifications мертвы в обеих ветках; hotkeys не было нигде.

## План этапов

1. Go: side-эффекты в `SaveSettings` + эмиты theme/locale; убрать
   оркестрацию из SettingsPage (отклонение 1).
2. Go: API-поллер + событие `api:changed`; MainPage на событие, таймер
   выпилить (отклонение 2).
3. Go: история трафика в `traffic:updated` + честный
   `traffic_graph_history` (отклонение 3).
4. Go: `log.limit` в бэкенд-буфере; фронт снапшот + доклейка (отклонение 4).
5. Payload конфигов: `cached`, `style`, `hash_mismatch`, `next_update`;
   ConfigsPage рендерит бейджи/цвета как в Gio (отклонение 6 + секция выше).
6. Стартовая оркестрация: Go шлёт theme/locale когда готов; убрать
   `setTimeout`-хак (отклонение 7).
7. Решить StartupPage (отклонение 5 / открытое решение 3).
8. Доступность CorePage (вернуть в навигацию или вкладку Settings→Core) +
   слушатель `core:update_available` + toast/диалог при старте для
   `selfupdate:available` (аудит, «критичное»).
9. Мёртвые настройки: применять `show_logs` (навигация), портировать
   `log.level` (DTO + фильтр в `GetAppLogs`).
10. Конфиги-формы: чекбокс auto_update, скрытие URL для local, read-only
    тип в edit, дефолт периода из `updates.default_interval_hours`.
11. Main-страница: uptime, сводка url-test (avg/count), click-to-copy в
    деталях соединения, детект смерти ядра до готовности API.
12. По мелочи: навигация при OnCoreMissing/OnConfigMissing, ченджлог в
    self-update confirm, заголовок окна из i18n, minimize-vs-hide в трее.
13. Опционально: re-exec для ResetData; KV-настройки; плагины.
