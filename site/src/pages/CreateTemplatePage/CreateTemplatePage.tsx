import { useMachine } from "@xstate/react"
import { Maybe } from "components/Conditionals/Maybe"
import { FullPageHorizontalForm } from "components/FullPageForm/FullPageHorizontalForm"
import { Loader } from "components/Loader/Loader"
import { useOrganizationId } from "hooks/useOrganizationId"
import { FC } from "react"
import { Helmet } from "react-helmet-async"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router-dom"
import { pageTitle } from "util/page"
import { createTemplateMachine } from "xServices/createTemplate/createTemplateXService"
import { CreateTemplateForm } from "./CreateTemplateForm"

const CreateTemplatePage: FC = () => {
  const { t } = useTranslation("createTemplatePage")
  const navigate = useNavigate()
  const organizationId = useOrganizationId()
  const [searchParams] = useSearchParams()
  const [state, send] = useMachine(createTemplateMachine, {
    context: {
      organizationId,
      exampleId: searchParams.get("exampleId"),
    },
    actions: {
      onCreate: (_, { data }) => {
        navigate(`/templates/${data.name}`)
      },
    },
  })
  const { starterTemplate, parameters, error, file } = state.context
  const shouldDisplayForm = !state.hasTag("loading")

  const onCancel = () => {
    navigate(-1)
  }

  return (
    <>
      <Helmet>
        <title>{pageTitle(t("title"))}</title>
      </Helmet>

      <FullPageHorizontalForm title={t("title")} onCancel={onCancel}>
        <Maybe condition={state.hasTag("loading")}>
          <Loader />
        </Maybe>

        {shouldDisplayForm && (
          <CreateTemplateForm
            error={error}
            starterTemplate={starterTemplate}
            isSubmitting={state.hasTag("submitting")}
            parameters={parameters}
            onCancel={onCancel}
            onSubmit={(data) => {
              send({
                type: "CREATE",
                data,
              })
            }}
            upload={{
              file,
              isUploading: state.matches("uploading"),
              onRemove: () => {
                send("REMOVE_FILE")
              },
              onUpload: (file) => {
                send({ type: "UPLOAD_FILE", file })
              },
            }}
          />
        )}
      </FullPageHorizontalForm>
    </>
  )
}

export default CreateTemplatePage
