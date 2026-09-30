import type { autofill as english } from "../en/autofill.ts";

export const autofill: typeof english = {
  "autofill.entry.unlock": "Разблокировать",
  "autofill.entry.search": "Искать в Ravenpass",
  "autofill.provider.subtitle": "Пароли, ключи доступа, коды",
  "autofill.unlock.description":
    "Разблокируйте хранилище, чтобы подставлять пароли.",
  "autofill.verify.description":
    "Введите пин-код, чтобы подтвердить, что это вы.",
  "autofill.search.title": "Выберите пароль",
  "autofill.search.code-title": "Выберите одноразовый код",
  "autofill.search.passkey-title": "Выберите ключ доступа или пароль",
  "autofill.search.passkey-only-title": "Выберите ключ доступа",
  "autofill.search.no-passkeys":
    "Здесь появятся ключи доступа, которые вы сохраните для этого сайта.",
  "autofill.search.passkeys": "Ключи доступа",
  "autofill.search.for-site": "Для этого сайта",
  "autofill.search.for-app": "Для этого приложения",
  "autofill.search.everything": "Все пароли",
  "autofill.search.error":
    "Не удалось выполнить поиск паролей. Повторите попытку.",
  "autofill.add-site.title": "Добавить {site} в «{label}»?",
  "autofill.add-site.detail":
    "Теперь вы сможете подставлять эту запись на сайте {site}.",
  "autofill.add-site.confirm": "Добавить и подставить",
  "autofill.add-site.error":
    "Не удалось добавить сайт в запись. Повторите попытку.",
  "autofill.link.title": "Использовать в этом приложении?",
  "autofill.link.detail":
    "Теперь Ravenpass будет предлагать «{label}» в этом приложении.",
  "autofill.link.confirm": "Использовать",
  "autofill.link.error":
    "Не удалось связать приложение с записью. Повторите попытку.",
  "autofill.no-code": "В этой записи нет одноразового кода. Выберите другую.",
  "autofill.cancel": "Отмена",
  "autofill.close": "Закрыть",
  "autofill.wait.opening": "Открываем Ravenpass…",
  "autofill.wait.unlock":
    "Подтвердите в Ravenpass, что это вы: Touch ID, пароль от Mac или пин-код.",
  "autofill.wait.verify.title": "Подтвердите, что это вы",
  "autofill.wait.verify":
    "Используйте Touch ID или пароль от Mac либо введите пин-код в Ravenpass.",
  "autofill.failed.title": "Что-то пошло не так",
  "autofill.failed.unreachable":
    "Ravenpass не отвечает. Откройте Ravenpass и повторите попытку.",
  "autofill.failed.outdated":
    "Ravenpass обновился, пока был открыт. Перезапустите Ravenpass и повторите попытку.",
  "autofill.failed.not-found": "Этой записи больше нет в хранилище.",
  "autofill.failed.sign-in":
    "Не удалось войти с этим ключом доступа. Повторите попытку.",
  "autofill.failed.save-passkey":
    "Не удалось сохранить ключ доступа. Повторите попытку.",
  "autofill.failed.unverifiable":
    "Этот сайт просит подтвердить, что это вы. Задайте пин-код или включите биометрию в Ravenpass и повторите попытку.",
  "autofill.failed.unsupported":
    "Ravenpass не может создать ключ доступа нужного этому сайту типа. Выберите другой способ входа.",
  "autofill.failed.other":
    "Не удалось выполнить этот запрос. Повторите попытку.",
};
