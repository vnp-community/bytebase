// i18n: i18next | use t("key") from useTranslation()
import { IssueDetailAccessGrantDetails } from "./IssueDetailAccessGrantDetails";

export function IssueDetailAccessGrantView() {
  return (
    <div className="flex w-full flex-col gap-y-4">
      <IssueDetailAccessGrantDetails />
    </div>
  );
}
