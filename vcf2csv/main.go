// vcf2csv converts a vcard file to csv.  This is designed very specifically
// for Calvary Chapel Half Moon Bay's church directory, and is unlikely to be
// useful for other purposes without heavy modification.
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	vcard "github.com/emersion/go-vcard"
)

var debug = flag.Bool("debug", false, "print extra debug statements")

func main() {
	flag.Parse()

	cards, err := readCards(os.Stdin)
	if err != nil {
		log.Fatalf("error reading from stdin: %v", err)
	}
	fmt.Fprintf(os.Stderr, "found %d cards\n", len(cards))

	for _, card := range cards {
		entry, err := convertCard(card)
		if err != nil {
			log.Fatalf("error converting card: %v", err)
		}

		record := []string{
			entry.GivenName,
			entry.FamilyName,
			entry.Image,
			strings.Join(entry.Address, "\n\n"),
			strings.Join(entry.Phone, "\n"),
			strings.Join(entry.Email, "\n"),
			strings.Join(entry.Birthday, "\n"),
			strings.Join(entry.Children, "\n"),
			entry.Anniversary,
		}

		if *debug {
			fmt.Print("csv record: ")
		}
		w := csv.NewWriter(os.Stdout)
		if err := w.Write(record); err != nil {
			log.Fatalf("error writing csv record: %v", err)
		}
		w.Flush()
	}
}

// readCards reads all vcards from r.
func readCards(r io.Reader) ([]vcard.Card, error) {
	dec := vcard.NewDecoder(r)
	var cards []vcard.Card
	for {
		card, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			return cards, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

// convertCard converts a single vcard into a csv row.
func convertCard(card vcard.Card) (Entry, error) {
	if *debug {
		fmt.Fprintf(os.Stderr, "\n%v %v\n", card.Name().GivenName, card.Name().FamilyName)
	}
	var e Entry

	// labels map group names to string labels
	var labels = make(map[string]string)
	for _, v := range card["X-ABLABEL"] {
		labels[v.Group] = normalizeLabel(v.Value)
	}
	delete(card, "X-ABLABEL")

	// extract anniversary date from custom apple field
	for _, v := range card["X-ABDATE"] {
		if labels[v.Group] == LabelAnniversary {
			if e.Anniversary != "" {
				return e, fmt.Errorf("duplicate anniversary value: %v", v)
			}
			e.Anniversary = formatDate(v.Value)
		} else {
			return e, fmt.Errorf("unknown date value: %v", v)
		}
	}
	delete(card, "X-ABDATE")

	// extract primary and spouse given names
	var primaryName, spouseName string
	nameParts := strings.Split(card.Name().GivenName, "&")
	primaryName = strings.TrimSpace(nameParts[0])
	if len(nameParts) > 1 {
		spouseName = strings.TrimSpace(nameParts[1])
	}

	if v := card[vcard.FieldName]; len(v) != 1 {
		return e, fmt.Errorf("expected 1 name, found %v: %v", len(v), v)
	}
	e.GivenName = card.Name().GivenName
	e.FamilyName = card.Name().FamilyName
	delete(card, vcard.FieldName)
	delete(card, vcard.FieldFormattedName)

	// address
	for _, a := range card.Addresses() {
		if a.PostOfficeBox != "" || a.ExtendedAddress != "" {
			return e, fmt.Errorf("address has additional information: %v", a)
		}
		address := fmt.Sprintf("%v\n%v, %v %v", a.StreetAddress, a.Locality, a.Region, a.PostalCode)
		e.Address = append(e.Address, address)
	}
	delete(card, vcard.FieldAddress)

	// birthdays
	if v := card.Value(vcard.FieldBirthday); v != "" {
		bday := formatDate(v)
		if spouseName != "" {
			bday = fmt.Sprintf("%v: %v", primaryName, bday)
		}
		e.Birthday = append(e.Birthday, bday)
		delete(card, vcard.FieldBirthday)
	}
	if fields := card["X-ABRELATEDNAMES"]; len(fields) > 0 {
		var unused []*vcard.Field
		for _, f := range fields {
			switch labels[f.Group] {
			case LabelBirthday, LabelPartner, LabelSpouse:
				e.Birthday = append(e.Birthday, formatBirthday(f.Value))
			case LabelChild:
				e.Children = append(e.Children, formatBirthday(f.Value))
			case LabelAnniversary:
				if e.Anniversary != "" {
					return e, fmt.Errorf("duplicate anniversary value: %v", f)
				}
				e.Anniversary = formatDate(f.Value)
			default:
				unused = append(unused, f)
			}
		}
		card["X-ABRELATEDNAMES"] = unused
	}

	for _, v := range card[vcard.FieldEmail] {
		e.Email = append(e.Email, formatField(v, nil))
	}
	delete(card, vcard.FieldEmail)

	for _, v := range card[vcard.FieldTelephone] {
		e.Phone = append(e.Phone, formatField(v, formatPhone))
	}
	delete(card, vcard.FieldTelephone)

	if *debug {
		fmt.Printf("labels: %v\n", labels)
		unusedBuf := bytes.NewBuffer(nil)
		for k, v := range card {
			if _, skip := ignoredFields[k]; skip {
				continue
			}
			for _, w := range v {
				fmt.Fprintf(unusedBuf, "  %v: %#v\n", k, w)
			}
		}
		if unusedBuf.Len() > 0 {
			fmt.Printf("unused fields:\n%v", unusedBuf.String())
		}
	}

	return e, nil
}

// normalizeLabel cleans Apple's X-ABLABEL values.  Apple standard values are
// of the form "_$!<Name>!$_", while user supplied values have no special
// syntax.  For our purposes, we don't care to distinguish between these, so
// remove the special syntax.
func normalizeLabel(v string) string {
	v = strings.TrimPrefix(v, "_$!<")
	v = strings.TrimSuffix(v, ">!$_")
	return v
}

// formateBirthday converts "Value - Name" to "Name: Value".
func formatValue(s string, fn func(string) string) string {
	parts := strings.Split(s, " - ")
	if len(parts) == 2 {
		value := strings.TrimSpace(parts[0])
		if fn != nil {
			value = fn(value)
		}
		name := strings.TrimSpace(parts[1])
		return fmt.Sprintf("%v: %v", name, value)
	}
	if fn != nil {
		return fn(s)
	}
	return s
}

func formatField(f *vcard.Field, fn func(string) string) string {
	val := formatValue(f.Value, fn)
	for _, t := range f.Params["TYPE"] {
		switch t {
		case "VOICE", "INTERNET", "pref":
			continue
		default:
			val = fmt.Sprintf("%v (%v)", val, strings.ToLower(t))
		}
	}
	return val
}

// formateBirthday converts "1/2 - Name" to "Name: Jan 2".
func formatBirthday(s string) string {
	return formatValue(s, formatDate)
}

// formatDate converts dates of the form "1/2" or "2006-01-02" into "Jan 2".
func formatDate(s string) string {
	for _, f := range []string{"1/2", "2006-01-02", "Jan _2, 2006"} {
		if t, err := time.Parse(f, s); err == nil {
			return t.Format("Jan 2")
		}
	}
	return s
}

func formatPhone(s string) string {
	var num string
	for _, r := range s {
		if '0' <= r && r <= '9' {
			num = num + string(r)
		}
	}
	if len(num) == 10 {
		return fmt.Sprintf("(%v) %v-%v", num[0:3], num[3:6], num[6:10])
	}
	return s
}

// vcard fields that we don't care about
var ignoredFields = map[string]bool{
	"UID":        true,
	"VERSION":    true,
	"CATEGORIES": true,
}

// Values for X-ABLABEL fields
var (
	LabelAnniversary = "Anniversary"
	LabelBirthday    = "Birthday"
	LabelChild       = "Child"
	LabelPartner     = "Partner"
	LabelSpouse      = "Spouse"
)

// Entry represents an individual or family in the directory.
type Entry struct {
	GivenName   string
	FamilyName  string
	Image       string
	Address     []string
	Phone       []string
	Email       []string
	Birthday    []string
	Children    []string
	Anniversary string
}
