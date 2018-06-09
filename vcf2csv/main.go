// vcf2csv convert a vcard file to csv.
package main

import (
	"io"
	"log"
	"os"

	vcard "github.com/emersion/go-vcard"
)

func main() {
	cards, err := readCards(os.Stdin)
	if err != nil {
		log.Fatalf("error reading from stdin: %v", err)
	}
	fields := map[string]int{}

	for _, card := range cards {
		for k, v := range card {
			if _, skip := ignoredFields[k]; skip {
				continue
			}
			for _, w := range v {
				log.Printf("%v: %#v", k, w)
			}
			if fields[k] < len(v) {
				fields[k] = len(v)
			}
		}
	}

	log.Printf("fields: %v", fields)
}

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

var ignoredFields = map[string]bool{
	"PRODID":  true,
	"VERSION": true,
	"UID":     true,
}
