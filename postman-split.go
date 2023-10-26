package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"os"

	postman "github.com/rbretecher/go-postman-collection"
)

type Info struct {
	Name        string              `json:"name"`
	Description postman.Description `json:"description"`
	Version     string              `json:"version"`
	Schema      string              `json:"schema"`
	PostmanID   string              `json:"_postman_id"`
}

type CustomCollection struct {
	Auth      *postman.Auth       `json:"auth,omitempty"`
	Info      Info                `json:"info"`
	Items     []*postman.Items    `json:"item"`
	Events    []*postman.Event    `json:"event,omitempty"`
	Variables []*postman.Variable `json:"variable,omitempty"`
}

type SharedCollection struct {
	Collection *postman.Collection `json:"collection"`
}

func createFolderChunks(originalArray []*postman.Items, minItemsPerSubarray int) [][]*postman.Items {
	numItems := len(originalArray)
	numSubarrays := (numItems + minItemsPerSubarray - 1) / minItemsPerSubarray

	result := make([][]*postman.Items, numSubarrays)

	for i := 0; i < numSubarrays; i++ {
		startIdx := i * minItemsPerSubarray
		endIdx := startIdx + minItemsPerSubarray

		if endIdx > numItems {
			endIdx = numItems
		}

		subarray := originalArray[startIdx:endIdx]

		result[i] = subarray
	}

	return result
}

func readFromURL(url *string) *SharedCollection {
	resp, err := http.Get(*url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP request failed with status code: %v\n", resp.Status)
		return nil
	}

	c, err := ioutil.ReadAll(resp.Body) //<--- here!
	var result *SharedCollection

	if err := json.Unmarshal(c, &result); err != nil { // Parse []byte to go struct pointer
		fmt.Println("Can not unmarshal JSON")
	}

	return result
}

func readFromFile(path *string) *postman.Collection {
	file, err := os.Open(*path)
	defer file.Close()

	if err != nil {
		panic(err)
	}

	c, err := postman.ParseCollection(file)

	return c
}

func main() {
	source := flag.String("collection", "", "A Postman collection to Parse")
	output := flag.String("output", "", "Result path")
	maxRequests := flag.Int("maxRequests", 10, "Max requests number in each folder")
	url := flag.String("url", "", "A Postman collection URL")
	postmanId := flag.String("postmanId", "", "A Postman collection ID")

	flag.Parse()

	items := []*postman.Items{}

	if *url != "" {
		c := readFromURL(url)

		items = c.Collection.Items
	}

	if *source != "" && *url == "" {
		c := readFromFile(source)

		items = c.Items
	}

	for _, ptr := range items {
		value := *ptr

		if value.IsGroup() {
			chunks := createFolderChunks(value.Items, *maxRequests)

			for i, chunk := range chunks {
				testName := value.Name

				if i > 0 {
					testName = testName + fmt.Sprintf("-%v", i)
				}

				c := postman.CreateCollection(testName, "")
				g := c.AddItemGroup(value.Name)

				for _, r := range chunk {
					g.AddItem(r)
				}

				path := fmt.Sprintf("%s/%s.json", *output, testName)

				file, err := os.Create(path)
				defer file.Close()

				if err != nil {
					panic((err))
				}

				err = c.Write(file, postman.V210)

				if err != nil {
					panic(err)
				}

				jsonData, err := ioutil.ReadFile(path)
				if err != nil {
					panic(err)
				}

				// Unmarshal the JSON data into a struct
				nc := CustomCollection{}

				if err := json.Unmarshal(jsonData, &nc); err != nil {
					fmt.Printf("Error unmarshaling JSON: %v\n", err)
					panic(err)
				}

				nc.Info.PostmanID = *postmanId

				newJSON, err := json.MarshalIndent(nc, "", "  ")
				if err != nil {
					fmt.Printf("Error marshaling JSON: %v\n", err)
					panic(err)
				}

				if err := ioutil.WriteFile(path, newJSON, 0644); err != nil {
					fmt.Printf("Error writing JSON to file: %v\n", err)
					panic(err)
				}
			}
		}
	}
}
