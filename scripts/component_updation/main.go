/*
Meshery Component Updater
Uses a spreadsheet of centralized information about MeshModel components and their metadata like color, icon, and so on. Script is used to update both components that are registered in thee Capabilities Registry and on the websites.

Usage: (order of flags matters)

    ./main [path-to-spreadsheet] [--update_doc] [relative path to docs in layer5 website] [relative path to docs in meshery website] [--only-published]

Example:

	./main https://docs.google.com/spreadsheets/d/e/2PACX-1vSgOXuiqbhUgtC9oNbJlz9PYpOEaFVoGNUFMIk4NZciFfQv1ewZg8ahdrWHKI79GkKK9TbmnZx8CqIe/pub\?gid\=0\&single\=true\&output\=csv --update-docs layer5/src/collections/integrations meshery.io/integrations --only-published

The flags are:

    --update-docs
        Skip updating components in meshery/meshery. Update website(s) only.

	--only-published
        Only handle components that have a value of "true" under the "Published?" column in spreadsheet.
*/

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/layer5io/component_scraper/pkg"
	"github.com/layer5io/meshkit/models/meshmodel/core/v1alpha1"
)

var (
	ColumnNamesToExtract        = []string{"model-display-name", "model", "category", "sub-category", "shape", "primary-color", "secondary-color", "logo-URL", "svg_color", "svg_white"}
	ColumnNamesToExtractForDocs = []string{"model-display-name", "Page Subtitle", "Docs URL", "category", "sub-category", "Feature 1", "Feature 2", "Feature 3", "howItWorks", "howItWorksDetails", "Publish?", "About Project", "Standard Blurb", "svg_color", "svg_white", "Full Page", "model"}
	PrimaryColumnName           = "model"
	OutputPath                  = "../../server/meshmodel/components"
)

const (
	SVG_WIDTH  = 20
	SVG_HEIGHT = 20
)

func main() {
	url := os.Args[1]
	if url == "" {
		log.Fatal("provide a valid URL")
		return
	}

	// If updateDocs is set true then it updates the website docs instead of updating the components onto the filesystem
	var updateDocs bool

	// If updateOnlyPublished is set true, then only update site pages that have "Published?" set to true.
	updateOnlyPublished := true

	var pathToIntegrationsLayer5 string
	var pathToIntegrationsMeshery string
	if len(os.Args) > 4 {
		if os.Args[2] == "--update-docs" {
			updateDocs = true
			pathToIntegrationsLayer5 = os.Args[3]
			pathToIntegrationsMeshery = os.Args[4]
			if len(os.Args) > 5 && os.Args[5] == "--only-published" {
				updateOnlyPublished = true
			}
		}
		if os.Args[2] == "--only-published" {
			updateOnlyPublished = true
		}
	}
	filep, err := pkg.DownloadCSV(url)
	if err != nil {
		log.Fatal(err)
		return
	}
	file, err := os.Open(filep)
	if err != nil {
		log.Fatal(err)
		return
	}
	csvReader := csv.NewReader(file)

	//*** UPDATE WEBSITE ***/
	if updateDocs {
		output, err := pkg.GetEntries(csvReader, ColumnNamesToExtractForDocs)
		if err != nil {
			log.Fatal(err)
			return
		}
		file.Close()
		os.Remove(file.Name())
		output = cleanupDuplicatesAndPreferEmptyComponentField(output, "model")
		mesheryDocsJSON := "["
		for _, out := range output {
			var t pkg.TemplateAttributes
			publishValue, err := strconv.ParseBool(out["Publish?"])
			if err != nil {
				publishValue = false
			}
			if updateOnlyPublished && !publishValue {
				continue
			}
			for key, val := range out {
				switch key {
				case "model-display-name":
					t.Title = val
				case "model":
					t.ModelName = val
				case "Page Subtitle":
					t.Subtitle = val
				case "Docs URL":
					t.DocURL = val
				case "category":
					t.Category = val
				case "sub-category":
					t.Subcategory = val
				case "howItWorks":
					t.HowItWorks = val
				case "hotItWorksDetails":
					t.HowItWorksDetails = val
				case "Publish?":
					t.Published = val
				case "About Project":
					t.AboutProject = val
				case "Standard Blurb":
					t.StandardBlurb = val
				case "Full Page":
					t.FullPage = val
				case "svg_color":
					t.ColorSVG = val
				case "svg_white":
					t.WhiteSVG = val
				}
			}
			t.FeatureList = "[" + strings.Join([]string{out["Feature 1"], out["Feature 2"], out["Feature 3"]}, ",") + "]"
			t.WorkingSlides = `[
				../_images/meshmap-visualizer.png,
				../_images/meshmap-designer.png]`

			md := t.CreateMarkDown()
			jsonItem := t.CreateJSONItem()
			// if out["model-display-name"] == "Istio" {
			// 	fmt.Println(md)
			// }
			mesheryDocsJSON += jsonItem + ","

			pathToIntegrationsLayer5, _ := filepath.Abs(filepath.Join("../../../", pathToIntegrationsLayer5, out["model"]))
			pathToIntegrationsMeshery, _ := filepath.Abs(filepath.Join("../../../", pathToIntegrationsMeshery))
			err = os.MkdirAll(pathToIntegrationsLayer5, 0777)
			if err != nil {
				panic(err)
			}
			_ = pkg.WriteToFile(filepath.Join(pathToIntegrationsLayer5, "index.mdx"), md)
			svgcolor := out["svg_color"]
			svgwhite := out["svg_white"]

			// Write SVGs to Layer5 docs
			err = os.MkdirAll(filepath.Join(pathToIntegrationsLayer5, "icon", "color"), 0777)
			if err != nil {
				panic(err)
			}

			err = pkg.WriteSVG(filepath.Join(pathToIntegrationsLayer5, "icon", "color", out["model"]+"-color.svg"), svgcolor) //CHANGE PATH
			if err != nil {
				panic(err)
			}
			err = os.MkdirAll(filepath.Join(pathToIntegrationsLayer5, "icon", "white"), 0777)
			if err != nil {
				panic(err)
			}
			err = pkg.WriteSVG(filepath.Join(pathToIntegrationsLayer5, "icon", "white", out["model"]+"-white.svg"), svgwhite) //CHANGE PATH
			if err != nil {
				panic(err)
			}

			// Write SVGs to Meshery docs
			err = os.MkdirAll(filepath.Join(pathToIntegrationsMeshery, "../", "images"), 0777)
			if err != nil {
				panic(err)
			}

			err = pkg.WriteSVG(filepath.Join(pathToIntegrationsMeshery, "../", "images", out["model"]+"-color.svg"), svgcolor) //CHANGE PATH
			if err != nil {
				panic(err)
			}

			err = pkg.WriteSVG(filepath.Join(pathToIntegrationsMeshery, "../", "images", out["model"]+"-white.svg"), svgwhite) //CHANGE PATH
			if err != nil {
				panic(err)
			}
		}

		mesheryDocsJSON = strings.TrimSuffix(mesheryDocsJSON, ",")
		mesheryDocsJSON += "]"
		if err := pkg.WriteToFile(filepath.Join("../../../", pathToIntegrationsMeshery, "data.json"), mesheryDocsJSON); err != nil {
			log.Fatal(err)
		}

	} else {
		output, err := pkg.GetEntries(csvReader, ColumnNamesToExtract)
		if err != nil {
			log.Fatal(err)
			return
		}
		file.Close()
		os.Remove(file.Name())
		err = pkg.PopulateEntries(OutputPath, output, PrimaryColumnName, func(dirpath string, changeFields map[string]string) error {
			entries, err := os.ReadDir(dirpath)
			if err != nil {
				return err
			}
			for _, versionentry := range entries {
				if versionentry.IsDir() {
					entries, err := os.ReadDir(filepath.Join(dirpath, versionentry.Name()))
					if err != nil {
						return err
					}
					for _, entry := range entries {
						name := strings.TrimSuffix(strings.TrimSpace(entry.Name()), ".json")
						if changeFields["Component"] != "" && changeFields["Component"] != name { //This is a component specific entry and only fill this when the filename matches component name
							continue
						}

						path, err := filepath.Abs(filepath.Join(dirpath, versionentry.Name(), entry.Name()))
						if err != nil {
							return err
						}
						byt, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						var component v1alpha1.ComponentDefinition
						err = json.Unmarshal(byt, &component)
						if err != nil {
							return err
						}
						fmt.Println("updating for", component.Kind)
						if component.Metadata == nil {
							component.Metadata = make(map[string]interface{})
						}
						for key, value := range changeFields {
							if key == "category" {
								component.Model.Category = value
							} else if key == "sub-category" {
								component.Model.SubCategory = value
							} else if key == "svg_color" || key == "svg_white" {
								component.Metadata[key], err = pkg.UpdateSVGString(value, SVG_WIDTH, SVG_HEIGHT)
								if err != nil {
									fmt.Println("err for: ", component.Kind, err.Error())
								}
								// component.Metadata[key] = value
							} else if isInColumnNames(key, ColumnNamesToExtract) != -1 {
								component.Metadata[key] = value
							}
						}
						if i := isInColumnNames("model-display-name", ColumnNamesToExtract); i != -1 {
							component.Model.DisplayName = changeFields[ColumnNamesToExtract[i]]
						}
						byt, err = json.Marshal(component)
						if err != nil {
							return err
						}
						err = os.WriteFile(filepath.Join(dirpath, versionentry.Name(), entry.Name()), byt, 0777)
						if err != nil {
							return err
						}
					}
				}

			}
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}

}

// returns the index of column. Returns -1 if doesn't exist
func isInColumnNames(key string, col []string) int {
	for i, n := range col {
		if n == key {
			return i
		}
	}
	return -1
}

// For Docs:: entries with empty Component field are preferred as they are considered general
// In other words, the absence of a component name indicates that a given row is a Model-level entry.
// And that for docs/websites updates, the values found in this row should be used to represent the
// integration overall (whether there is 1 or many 10s of components contained in the package / in the integration).
func cleanupDuplicatesAndPreferEmptyComponentField(out []map[string]string, groupBykey string) (out2 []map[string]string) {
	keyToEntry := make(map[string]map[string]string)
	for _, o := range out {
		gkey := o[groupBykey]
		//If the row with given gkey is encountered for the first time, or the given row already exists but with a non-empty component field then use the new entry.
		//This logic will prioritize empty component fields to not be overriden
		if keyToEntry[gkey] == nil || keyToEntry[gkey]["Component"] != "" {
			fmt.Println("for model ", gkey, " will use ")
			keyToEntry[gkey] = o
		}

	}
	for _, entry := range keyToEntry {
		out2 = append(out2, entry)
	}
	return out2
}
