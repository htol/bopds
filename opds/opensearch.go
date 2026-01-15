package opds

import "encoding/xml"

// OpenSearchDescription represents an OpenSearch description document
type OpenSearchDescription struct {
	XMLName     xml.Name      `xml:"OpenSearchDescription"`
	Xmlns       string        `xml:"xmlns,attr"`
	ShortName   string        `xml:"ShortName"`
	Description string        `xml:"Description"`
	InputEnc    string        `xml:"InputEncoding"`
	OutputEnc   string        `xml:"OutputEncoding"`
	URL         OpenSearchURL `xml:"Url"`
}

// OpenSearchURL represents the URL template for OpenSearch
type OpenSearchURL struct {
	Type     string `xml:"type,attr"`
	Template string `xml:"template,attr"`
}

// NewOpenSearchDescription creates an OpenSearch description document
func NewOpenSearchDescription(baseURL, shortName, description string) *OpenSearchDescription {
	return &OpenSearchDescription{
		Xmlns:       NamespaceSearch,
		ShortName:   shortName,
		Description: description,
		InputEnc:    "UTF-8",
		OutputEnc:   "UTF-8",
		URL: OpenSearchURL{
			Type:     TypeAcquisition,
			Template: baseURL + "/opds/search?q={searchTerms}",
		},
	}
}

// Marshal returns the XML representation of the OpenSearch document
func (o *OpenSearchDescription) Marshal() ([]byte, error) {
	output, err := xml.MarshalIndent(o, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), output...), nil
}
