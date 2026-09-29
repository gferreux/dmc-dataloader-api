package catalog

import "github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"

func advertiserCustomers() model.Template {
	base := optinColumns()
	columns := make([]model.TemplateColumn, 0, len(base)+2)
	columns = append(columns, base...)
	columns = append(columns,
		col("country", model.BQString, "Country code.", "", false),
		col("additional_fields", model.BQJSON, "Extra key/values for reporting drilldown.", hintJSON, false),
	)

	return model.Template{
		PartnerType: model.PartnerAdvertiser,
		ImportType:  model.ImportCustomers,
		Label:       "Advertiser customers",
		Columns:     columns,
		Defaults: defaults(
			model.PartnerAdvertiser,
			model.ImportCustomers,
			datasetAdvertisers,
			"customers",
			columns,
		),
	}
}

func advertiserSales() model.Template {
	columns := []model.TemplateColumn{
		col("order_ts", model.BQTimestamp, "Order timestamp.", hintDate, true),
		col("order_id", model.BQString, "Order identifier.", "", false),
		col("external_customer_id", model.BQString, "Customer id in the advertiser system.", "", false),
		col("external_customer_name", model.BQString, "Customer name in the advertiser system.", "", false),
		col("mobile_phone", model.BQString, "Mobile phone, clear or hashed.", hintMobile, true),
		col("land_phone", model.BQString, "Landline phone.", "", false),
		col("email", model.BQString, "Email address.", "", false),
		col("sales_channel", model.BQString, "Sales channel.", "", false),
		col("store_id", model.BQString, "Store identifier.", "", true),
		col("store_name", model.BQString, "Store name.", "", false),
		col("price_before_tax", model.BQFloat, "Order price before tax.", "", true),
		col("price_with_tax", model.BQFloat, "Order price with tax.", "", false),
		col("item_id", model.BQString, "Item identifier.", "", false),
		col("item_name", model.BQString, "Item name.", "", false),
		col("item_description", model.BQString, "Item description.", "", false),
		col("item_category", model.BQString, "Item category.", "", false),
		col("item_quantity", model.BQInteger, "Item quantity.", "", false),
		col("item_price", model.BQFloat, "Item price.", "", false),
		col("additional_fields", model.BQJSON, "Extra key/values for reporting drilldown.", hintJSON, false),
	}

	return model.Template{
		PartnerType: model.PartnerAdvertiser,
		ImportType:  model.ImportSales,
		Label:       "Advertiser sales",
		Columns:     columns,
		Defaults: defaults(
			model.PartnerAdvertiser,
			model.ImportSales,
			datasetAdvertisers,
			"sales",
			columns,
		),
	}
}

func advertiserStores() model.Template {
	columns := []model.TemplateColumn{
		col("id", model.BQString, "Store identifier.", "", true),
		col("name", model.BQString, "Store name.", "", true),
		col("address", model.BQString, "Street address.", "", true),
		col("zip_code", model.BQString, "Postal code.", "", true),
		col("city", model.BQString, "City.", "", true),
		col("country", model.BQString, "Country code. Use FR when the file has no country.", "Default FR.", false),
		col("longitude", model.BQFloat, "Longitude in decimal degrees.", "", false),
		col("latitude", model.BQFloat, "Latitude in decimal degrees.", "", false),
		col("website", model.BQString, "Store website.", "", false),
		col("email", model.BQString, "Store email.", "", false),
		col("phone_number", model.BQString, "Store phone number.", "", false),
		col("tags", model.BQString, "Store tags.", hintTags, false),
		col("additional_fields", model.BQJSON, "Extra key/values for reporting drilldown.", hintJSON, false),
	}

	return model.Template{
		PartnerType: model.PartnerAdvertiser,
		ImportType:  model.ImportStores,
		Label:       "Advertiser stores",
		Columns:     columns,
		Defaults: defaults(
			model.PartnerAdvertiser,
			model.ImportStores,
			datasetAdvertisers,
			"stores",
			columns,
		),
	}
}

func advertiserBlacklists() model.Template {
	columns := []model.TemplateColumn{
		col("mobile_phone", model.BQString, "Mobile phone, clear or hashed.", hintMobile, true),
		col("email", model.BQString, "Email address.", "", false),
		col("land_phone", model.BQString, "Landline phone.", "", false),
		col("listed_date", model.BQDate, "Date the entry was added to the blacklist.", hintDate, false),
	}

	return model.Template{
		PartnerType:       model.PartnerAdvertiser,
		ImportType:        model.ImportBlacklists,
		Label:             "Advertiser blacklists",
		NeedsConfirmation: true,
		ConfirmationNote: "Blacklist columns could not be read from dev Firestore or the loader. " +
			"Confirm this placeholder with Greg before using it.",
		Columns: columns,
		Defaults: defaults(
			model.PartnerAdvertiser,
			model.ImportBlacklists,
			datasetAdvertisers,
			"blacklists",
			columns,
		),
	}
}
