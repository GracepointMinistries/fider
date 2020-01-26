package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/GracepointMinistries/membership/pkg/membership"
)

func getMembership(m map[string]string) error {
	people, err := membershipClient.GetPeopleAttributes(context.Background(), []string{"gpmail", "name"}, map[string][]string{"composite_ministry": []string{"berk_college"}})
	if err != nil {
		return fmt.Errorf("unable to get membership info %v", err)
	}

	for _, v := range people {
		if v.Attrs[0].AttrName == "gpmail" {
			email, _ := v.Attrs[0].Values[0].(string)
			name, _ := v.Attrs[1].Values[0].(string)
			m[email] = name
		} else {
			email, _ := v.Attrs[1].Values[0].(string)
			name, _ := v.Attrs[0].Values[0].(string)
			m[email] = name
		}
	}
	return nil
}

type FiderFolks struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

var FiderURL string
var FiderAPIKey string

func getExisting(e map[string]bool) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/users", FiderURL), nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", FiderAPIKey))
	req.Header.Add("Accept", "application/json")
	resp, err := fiderClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	info, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d, %s", resp.StatusCode, string(info))
	}

	ff := []FiderFolks{}
	err = json.Unmarshal(info, &ff)
	if err != nil {
		return fmt.Errorf("fider unmarshal error %v", err)
	}
	for _, v := range ff {
		e[v.Name] = true
	}
	return nil
}

var membershipClient membership.Client
var fiderClient *http.Client

func main() {
	MembershipURL := os.Getenv("MEMBERSHIP_URL")
	MembershipAPIKey := os.Getenv("MEMBERSHIP_API_KEY")
	FiderURL = os.Getenv("FIDER_URL")        //"https://convo.gracepointonline.org/api/v1/users"
	FiderAPIKey = os.Getenv("FIDER_API_KEY") //"Bearer hdKsaFKPIwzsfjsPOm7kPMlt3Zv16UlqmOGIlvjjiRfy81NABN6rXC8OtmIcenV5"

	if MembershipURL == "" || MembershipAPIKey == "" || FiderURL == "" || FiderAPIKey == "" {
		log.Fatal("must specify MEMBERSHIP_URL, MEMBERSHIP_API_KEY, FIDER_URL and FIDER_API_KEY")
	}

	var err error
	membershipClient, err = membership.NewClient(MembershipURL, MembershipAPIKey)
	if err != nil {
		log.Fatalf("unable to create membership client: %v", err)
	}

	membership := make(map[string]string)
	if err := getMembership(membership); err != nil {
		log.Fatalf("membership error %v", err)
	}
	log.Printf("retrieved %d entries from membership", len(membership))

	fiderClient = &http.Client{}

	existing := make(map[string]bool)
	if err := getExisting(existing); err != nil {
		log.Fatalf("existing fetch failed %v", err)
	}
	log.Printf("retrieved %d entries from Fider", len(existing))

	// first add in new members
	for e, n := range membership {
		if _, ok := existing[n]; !ok {
			log.Printf("adding %s (%s)\n", n, e)

			type InsertFields struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
			insFields := InsertFields{Name: n, Email: e}
			mJson, err := json.Marshal(insFields)
			if err != nil {
				log.Fatalf("couldn't marshal for POST")
			}
			log.Printf("json is %s\n", string(mJson))
			req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/users", FiderURL), strings.NewReader(string(mJson)))
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", FiderAPIKey))
			req.Header.Add("Content-Type", "application/json")
			resp, err := fiderClient.Do(req)
			if err != nil {
				log.Fatalf("unable to POST %v", err)
			}
			defer resp.Body.Close()
			info, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalf("couldn't read in %v", err)
			}

			if resp.StatusCode != http.StatusOK {
				log.Fatalf("post returned %d, body %s\n", resp.StatusCode, string(info))
			}
		}
	}
	log.Printf("done adding in new members")

	// TODO: now deactivated old members

}
