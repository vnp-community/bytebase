// i18n: i18next | use t("key") from useTranslation()
import { useEffect } from "react";
import { router } from "@/router";
import { PROJECT_V1_ROUTE_ISSUES } from "@/router/dashboard/projectV1";

export function ProjectLandingPage({ projectId }: { projectId: string }) {
  useEffect(() => {
    router.replace({
      name: PROJECT_V1_ROUTE_ISSUES,
      params: { projectId },
    });
  }, [projectId]);

  return <div />;
}
