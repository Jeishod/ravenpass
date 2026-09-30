import type { imports as english } from "../en/imports.ts";

export const imports: typeof english = {
  "settings.import.heading": "Импорт",
  "settings.import.stage.file": "Файл экспорта",
  "settings.import.stage.review": "Проверка",
  "settings.import.stage.result": "Результат",
  "settings.import.summary":
    "Выберите файл экспорта из другого менеджера паролей.",
  "settings.import.sources": "Все приложения",
  "settings.import.steps": "Как сделать экспорт",
  "settings.import.bitwarden.formats": ".json, .zip или .csv",
  "settings.import.bitwarden.step.open": "Откройте экспорт",
  "settings.import.bitwarden.step.open.detail":
    "«Инструменты» → «Экспорт» в веб-версии, «Настройки» → «Настройки хранилища» → «Экспорт» в расширении браузера или «Экспорт» в приложении для компьютера.",
  "settings.import.bitwarden.step.format": "Выберите формат файла",
  "settings.import.bitwarden.step.format.detail":
    "Файлы .json и .zip содержат все записи, .csv — только логины и заметки. Для зашифрованного .json выберите «Защищено паролем».",
  "settings.import.bitwarden.review":
    "Дополнительные поля и сайты попадут в заметки записей. Избранное останется в избранном. История паролей останется в {source}.",
  "settings.import.aliasvault.formats": ".avex, .avux или .csv",
  "settings.import.aliasvault.step.open": "Откройте экспорт",
  "settings.import.aliasvault.step.open.detail":
    "«Настройки» → «Импорт / экспорт» в веб-версии AliasVault. Мобильное приложение экспортирует только CSV — там же, в «Настройки» → «Импорт / экспорт».",
  "settings.import.aliasvault.step.format": "Выберите формат файла",
  "settings.import.aliasvault.step.format.detail":
    "Зашифрованный экспорт всего хранилища (.avex) содержит все записи и требует пароль, заданный при экспорте. Незашифрованный экспорт (.avux) содержит те же записи без пароля. CSV не включает дополнительные поля, теги и дополнительные одноразовые коды.",
  "settings.import.aliasvault.review":
    "Дополнительные поля, данные псевдонимов, а также дополнительные сайты и одноразовые коды попадут в заметки записей. История паролей останется в {source}.",
  "settings.import.folders": "Из папок {source}",
  "settings.import.folders-tags": "Из папок и тегов {source}",
  "settings.import.choose": "Выбрать файл…",
  "settings.import.reading": "Читаем…",
  "settings.import.note":
    "Ravenpass читает файл на этом устройстве и не изменяет его.",
  "settings.import.password.title": "Введите пароль файла",
  "settings.import.password.detail":
    "Файл {name} защищён паролем, который вы задали при экспорте.",
  "settings.import.password.label": "Пароль файла",
  "settings.import.password.empty": "Введите пароль, заданный при экспорте.",
  "settings.import.password.cancel": "Отмена",
  "settings.import.password.continue": "Продолжить",
  "settings.import.password.opening": "Открываем…",
  "settings.import.failed.title": "Не удаётся импортировать этот файл",
  "settings.import.failed.choose": "Выбрать другой файл…",
  "settings.import.file.detail":
    "{format} · {count, plural, one {# запись} few {# записи} many {# записей} other {# записи}}",
  "settings.import.format.json": "Файл JSON",
  "settings.import.format.encrypted-json": "JSON с паролем",
  "settings.import.format.zip": "Архив ZIP",
  "settings.import.format.encrypted-zip": "Архив с паролем",
  "settings.import.format.csv": "Файл CSV",
  "settings.import.change": "Сменить…",
  "settings.import.adding": "Будет добавлено",
  "settings.import.in-file": "{count} в файле",
  "settings.import.duplicates.skipped":
    "{count, plural, one {# запись уже есть в Ravenpass и будет пропущена} few {# записи уже есть в Ravenpass и будут пропущены} many {# записей уже есть в Ravenpass и будут пропущены} other {# записи уже есть в Ravenpass и будут пропущены}}",
  "settings.import.duplicates.kept":
    "{count, plural, one {# запись уже есть в Ravenpass} few {# записи уже есть в Ravenpass} many {# записей уже есть в Ravenpass} other {# записи уже есть в Ravenpass}}",
  "settings.import.one-time-codes":
    "{count, plural, one {# запись с одноразовым кодом} few {# записи с одноразовыми кодами} many {# записей с одноразовыми кодами} other {# записи с одноразовыми кодами}}",
  "settings.import.from.login":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из логинов",
  "settings.import.from.alias":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из псевдонимов",
  "settings.import.from.card":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из карт",
  "settings.import.from.identity":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из профилей",
  "settings.import.from.passport":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из паспортов",
  "settings.import.from.drivers-license":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из водительских удостоверений",
  "settings.import.from.note":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из защищённых заметок",
  "settings.import.from.ssh-key":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из SSH-ключей",
  "settings.import.from.bank-account":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}} из банковских счетов",
  "settings.import.option.groups": "Создать группы из папок",
  "settings.import.option.groups.detail":
    "Каждая запись попадёт в группу с названием своей папки.",
  "settings.import.option.groups-tags": "Создать группы из папок и тегов",
  "settings.import.option.groups-tags.detail":
    "Каждая запись попадёт в группы с названиями своей папки и тегов.",
  "settings.import.option.skip":
    "Пропускать записи, которые уже есть в Ravenpass",
  "settings.import.option.skip.detail":
    "Записи с таким же названием и совпадающими данными будут пропущены.",
  "settings.import.left": "Не будет перенесено",
  "settings.import.origin.login": "Логин",
  "settings.import.origin.alias": "Псевдоним",
  "settings.import.origin.card": "Карта",
  "settings.import.origin.identity": "Профиль",
  "settings.import.origin.passport": "Паспорт",
  "settings.import.origin.drivers-license": "Водительское удостоверение",
  "settings.import.origin.note": "Защищённая заметка",
  "settings.import.origin.ssh-key": "SSH-ключ",
  "settings.import.origin.bank-account": "Банковский счёт",
  "settings.import.origin.unknown": "Неизвестная запись",
  "settings.import.reason.too-long":
    "Одно или несколько полей превышают допустимую длину.",
  "settings.import.reason.unnamed": "Нет названия.",
  "settings.import.reason.unsupported":
    "Ravenpass не поддерживает этот тип записей.",
  "settings.import.more": "И ещё {count}",
  "settings.import.attachments":
    "{count, plural, one {# вложение} few {# вложения} many {# вложений} other {# вложения}}",
  "settings.import.attachments.detail": "Они останутся в файле экспорта.",
  "settings.import.passkeys":
    "{count, plural, one {# ключ доступа} few {# ключа доступа} many {# ключей доступа} other {# ключа доступа}}",
  "settings.import.passkeys.detail":
    "Логины будут импортированы без ключей доступа.",
  "settings.import.dropped":
    "{count, plural, one {# папка не станет группой} few {# папки не станут группами} many {# папок не станут группами} other {# папки не станут группами}}",
  "settings.import.dropped-tags":
    "{count, plural, one {# папка или тег не станет группой} few {# папки и тега не станут группами} many {# папок и тегов не станут группами} other {# папки и тега не станут группами}}",
  "settings.import.dropped.detail":
    "Их названия слишком длинные, или в Ravenpass уже максимум групп.",
  "settings.import.nothing":
    "Импортировать нечего: все записи уже есть в Ravenpass или не могут быть импортированы.",
  "settings.import.cancel": "Отмена",
  "settings.import.run": "Импортировать ({count})",
  "settings.import.importing": "Импортируем…",
  "settings.import.items":
    "{count, plural, one {# запись} few {# записи} many {# записей} other {# записи}}",
  "settings.import.importing.note":
    "Хранилище обновится за один шаг, когда все записи будут готовы.",
  "settings.import.done.title":
    "{count, plural, one {Добавлена # запись} few {Добавлено # записи} many {Добавлено # записей} other {Добавлено # записи}}",
  "settings.import.done.detail": "Из файла {name}",
  "settings.import.done.groups": "Новые группы",
  "settings.import.warning.title": "Удалите файл экспорта",
  "settings.import.warning.detail":
    "В нём ваши пароли в незашифрованном виде. Переместите его в Корзину, а затем очистите Корзину.",
  "settings.import.warning.zip":
    "В нём ваши пароли в незашифрованном виде. Сохраните нужные вложения в другом месте, затем переместите архив в Корзину и очистите её.",
  "settings.import.warning.manual":
    "В нём ваши пароли в незашифрованном виде, поэтому удалите его оттуда, где сохранили.",
  "settings.import.warning.zip.manual":
    "В нём ваши пароли в незашифрованном виде. Сохраните нужные вложения в другом месте, затем удалите архив оттуда, где сохранили.",
  "settings.import.trash": "Переместить в Корзину",
  "settings.import.reveal": "Показать в папке",
  "settings.import.trashed":
    "Файл экспорта в Корзине. Очистите Корзину, чтобы удалить его с этого устройства.",
  "settings.import.another": "Импортировать другой файл",
  "settings.import.error.choose":
    "Не удалось прочитать этот файл. Выберите его снова.",
  "settings.import.error.unlock":
    "Не удалось открыть этот файл. Выберите его снова.",
  "settings.import.error.run":
    "Не удалось завершить импорт. Прежде чем повторить, проверьте, добавились ли записи.",
  "settings.import.error.refresh":
    "Записи импортированы, но их не удалось показать. Заблокируйте и снова откройте хранилище, чтобы увидеть их.",
  "settings.import.error.trash":
    "Не удалось переместить файл в Корзину. Удалите его вручную.",
  "settings.import.error.reveal":
    "Не удалось открыть папку с файлом. Найдите файл и удалите его.",
  "settings.import.note.website": "Сайт",
  "settings.import.note.one-time-code": "Настройка одноразового кода",
  "settings.import.note.title": "Обращение",
  "settings.import.note.company": "Компания",
  "settings.import.note.username": "Имя пользователя",
  "settings.import.note.expiry": "Срок действия",
  "settings.import.note.security-code": "Код безопасности",
  "settings.import.note.card-number": "Номер карты",
  "settings.import.note.cardholder": "Держатель карты",
  "settings.import.note.brand": "Платёжная система",
  "settings.import.note.private-key": "Закрытый ключ",
  "settings.import.note.public-key": "Открытый ключ",
  "settings.import.note.fingerprint": "Отпечаток",
  "settings.import.note.bank": "Банк",
  "settings.import.note.account-holder": "Владелец счёта",
  "settings.import.note.account-type": "Тип счёта",
  "settings.import.note.account-number": "Номер счёта",
  "settings.import.note.routing-number": "Маршрутный номер",
  "settings.import.note.branch-number": "Номер отделения",
  "settings.import.note.pin": "Пин-код",
  "settings.import.note.swift": "SWIFT",
  "settings.import.note.iban": "IBAN",
  "settings.import.note.bank-phone": "Телефон банка",
  "settings.import.note.license-class": "Категория прав",
  "settings.import.note.sex": "Пол",
  "settings.import.note.birth-place": "Место рождения",
  "settings.import.note.nationality": "Гражданство",
  "settings.import.note.passport-type": "Тип паспорта",
  "settings.import.note.national-id": "Личный номер",
  "settings.import.note.issued-on": "Дата выдачи",
  "settings.import.note.expires-on": "Действует до",
  "settings.import.note.birthday": "Дата рождения",
  "settings.import.note.issuer": "Кем выдан",
  "settings.import.note.name": "Имя",
  "settings.import.note.gender": "Пол",
  "settings.import.note.nickname": "Никнейм",
};
