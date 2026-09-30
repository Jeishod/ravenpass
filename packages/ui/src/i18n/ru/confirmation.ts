import type { confirmation as english } from "../en/confirmation.ts";

export const confirmation: typeof english = {
  "confirmation.unlock.title": "Разблокировать Ravenpass",
  "confirmation.unlock.description":
    "Расширение браузера запрашивает доступ к хранилищу.",
  "confirmation.unlock.description.autofill":
    "Автозаполнение запрашивает доступ к хранилищу, чтобы выполнить вход.",
  "confirmation.change-unlock.title": "Изменить способ разблокировки?",
  "confirmation.change-unlock.description":
    "Введите текущий пин-код, чтобы подтвердить изменение.",
  "confirmation.change-unlock.confirm": "Подтвердить",
  "confirmation.change-unlock.error":
    "Не удалось подтвердить изменение. Повторите попытку.",
  "confirmation.decline": "Отклонить",
  "confirmation.decline-error":
    "Не удалось отклонить запрос. Повторите попытку.",
};
