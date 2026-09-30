import type { codes as english } from "../en/codes.ts";

export const codes: typeof english = {
  "code-setup.title": "Добавить одноразовый код",
  "code-setup.detail.issuer-account":
    "Выберите пароль для {account} на {issuer}.",
  "code-setup.detail.issuer": "Выберите пароль для {issuer}.",
  "code-setup.detail.account": "Выберите пароль для {account}.",
  "code-setup.detail.unnamed":
    "Выберите пароль, к которому относится этот код.",
  "code-setup.empty":
    "Ни один сохранённый пароль не подходит к этому коду. Найдите пароль или добавьте новый.",
  "code-setup.new": "Новый пароль",
  "code-setup.cancel": "Отмена",
  "code-setup.replace.title": "Заменить одноразовый код?",
  "code-setup.replace.detail":
    "В пароле «{label}» уже есть одноразовый код. Новый заменит его.",
  "code-setup.replace.confirm": "Заменить",
  "code-setup.added": "Одноразовый код добавлен.",
  "code-setup.error": "Не удалось добавить одноразовый код. Повторите попытку.",
  "code-setup.error.dismiss":
    "Не удалось отменить добавление одноразового кода. Ravenpass забудет его в течение пяти минут.",
};
