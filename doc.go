// Package cap implements the OASIS Common Alerting Protocol version 1.2.
//
// It provides a complete XML data model, bounded decoding, encoding, and
// semantic validation. Profile-specific validators live under profiles/ and
// can be composed with the base validator.
package cap

const (
	// Namespace is the XML namespace of OASIS CAP version 1.2.
	Namespace = "urn:oasis:names:tc:emergency:cap:1.2"
	// XMLDSigNamespace is the namespace permitted for signature extensions by
	// the CAP 1.2 schema.
	XMLDSigNamespace = "http://www.w3.org/2000/09/xmldsig#"
)
