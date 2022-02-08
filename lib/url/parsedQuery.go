package url

func (pq ParsedQuery) outputType() uint {
	if value, ok := pq.Table["output"]; ok {
		switch value {
		case "csv":
			return OutputTypeCsv
		case "json":
			return OutputTypeJson
		case "text":
			return OutputTypeText
		}
	}
	return OutputTypeHtml
}
