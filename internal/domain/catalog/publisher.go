package catalog

import "github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"

func publisherOptin() model.Template {
	columns := optinColumns()

	return model.Template{
		PartnerType: model.PartnerPublisher,
		ImportType:  model.ImportOptin,
		Label:       "Publisher opt-in (Bases)",
		Columns:     columns,
		Defaults:    defaults(model.PartnerPublisher, model.ImportOptin, datasetPublishers, "profiles", columns),
	}
}

func publisherOptout() model.Template {
	columns := []model.TemplateColumn{
		col("mobile_phone", model.BQString, "Mobile phone to opt out.", hintMobile, true),
		col("email", model.BQString, "Email address to opt out.", "", false),
		col("optout_date", model.BQDate, "Date the opt-out was recorded.", hintDate, true),
	}

	return model.Template{
		PartnerType:       model.PartnerPublisher,
		ImportType:        model.ImportOptout,
		Label:             "Publisher opt-out",
		NeedsConfirmation: true,
		ConfirmationNote: "No written spec was available for publisher opt-out. " +
			"Confirm the columns and the BigQuery table with Greg before using this template.",
		Columns: columns,
		Defaults: defaults(
			model.PartnerPublisher,
			model.ImportOptout,
			datasetPublishers,
			"optout",
			columns,
		),
	}
}

func optinColumns() []model.TemplateColumn {
	return []model.TemplateColumn{
		col("email", model.BQString, "Email address.", "", false),
		col("land_phone", model.BQString, "Landline phone.", "", false),
		col("mobile_phone", model.BQString, "Mobile phone.", hintMobile, true),
		col("id", model.BQString, "Contact identifier.", "", false),
		col("optin_email", model.BQString, "Email opt-in flag.", hintOptin, false),
		col("optin_sms", model.BQString, "SMS opt-in flag.", hintOptin, true),
		col("title", model.BQString, "Honorific title.", "", false),
		col("gender", model.BQString, "Gender.", hintGender, false),
		col("last_name", model.BQString, "Last name.", "", false),
		col("first_name", model.BQString, "First name.", "", false),
		col("birth_date", model.BQDate, "Birth date.", hintDate, false),
		col("address_1", model.BQString,
			"Street address line. Add address_3 and further lines as extra STRING mappings.", "", false),
		col("address_2", model.BQString, "Second street address line.", "", false),
		col("city", model.BQString, "City.", "", false),
		col("zip_code", model.BQString, "Postal code.", "", false),
		col("last_activity_date", model.BQDate, "Last activity date.", hintDate, false),
		col("collect_date", model.BQDate, "Date the contact was collected.", hintDate, true),
		col("collect_url", model.BQString, "URL where the contact was collected.", "", true),
		col("user_ip", model.BQString, "Collector IP address.", "", false),
	}
}
