package questions

import (
	"fmt"
	"os"
	"io/ioutil"
	"path/filepath"
	cmap "github.com/orcaman/concurrent-map"
	"encoding/json"
	"math/rand"
	"time"
)

type Question struct {
	QuestionText string `json:"question"`
	Correct string `json:"correct"`
	Incorrect []string `json:"incorrect"`
}

type Category struct {
	CategoryName string `json:"category"`
	Questions []Question `json:"questions"`
}

type QuestionsList struct {
	Categories []Category `json:"categories"`
}

type CatLoc struct {
	CategoryName string
	file string
}

var questionDir = filepath.Join("..", "game", "questions", "questions")

// TODO: does this need to be a cmap (does this need to be threadsafe?)
var categoryMap = cmap.New() 		// map category to the file it is located in

var catList []string				// list of category names
var CategoriesInitialized = false	// have we initialized the categories?

func PopulateCategories() {
	files, err := ioutil.ReadDir(questionDir)
	if err != nil {
		fmt.Println("ioutil.ReadDir failed in questions.PopulateCategories")
		return
	}

	for _, f := range files {
    
		catFile, err := os.Open(filepath.Join(questionDir, f.Name()))
		if err != nil {
			fmt.Println("os.Open failed in questions.PopulateCategories, filename = ", filepath.Join(questionDir, f.Name()))
			return
		}
    
		catFileBytes, err := ioutil.ReadAll(catFile)
		if err != nil {
			fmt.Println("ioutil.ReadAll failed in questions.PopulateCategories")
			return
		}
    
		data := QuestionsList{}
    
		err = json.Unmarshal(catFileBytes, &data)
		if err != nil {
			fmt.Println("json.Unmarshal failed in questions.PopulateCategories, err: ", err)
			return
		}
    
		for _, cat := range data.Categories {
			if len(cat.Questions) > 4 {
				categoryMap.Set(cat.CategoryName, filepath.Join(questionDir, f.Name()))
				catList = append(catList, cat.CategoryName)
			}
		}
	}
	
	CategoriesInitialized = true
}

func GetGameCategories() []string {

	// from the available categories, randomly select 6 to play with
	// TODO: kinda pointless when we only have six categories anyways
	// TODO (eventually): also randomly select questions (this function or somewhere else?)

	rand.Seed(time.Now().UnixNano())
	numCats := 6 // TODO: set this dynamically?
	var data []string
	p := rand.Perm(len(catList))
	for _, r := range(p[:numCats]) { 			// numCats[:6]
		data = append(data, catList[r])
	}

	return data

}

//func GetQuestion(pointVal uint8) game.Question {

	// get a question

//}









