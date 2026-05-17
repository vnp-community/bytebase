// i18n: i18next | use t("key") from useTranslation()
import { ProfileMenuTrigger } from "@/react/components/header/ProfileMenuTrigger";

export function HeaderProfileMenuMount({
  size = "small",
}: {
  size?: "small" | "medium";
}) {
  return <ProfileMenuTrigger size={size} link />;
}
