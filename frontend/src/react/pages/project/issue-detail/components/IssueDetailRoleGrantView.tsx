// i18n: i18next | use t("key") from useTranslation()
import { IssueDetailRoleGrantDetails } from "./IssueDetailRoleGrantDetails";

export function IssueDetailRoleGrantView() {
  return (
    <div className="flex w-full flex-col gap-y-4">
      <IssueDetailRoleGrantDetails />
    </div>
  );
}
