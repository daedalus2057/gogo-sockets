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
	Questions []*Question `json:"questions"`
}

type QuestionsList struct {
	Categories []*Category `json:"categories"`
}

type CatLoc struct {
	CategoryName string
	file string
}

var gameQuestions = cmap.New() // gameIds to their questions

var questionDir = filepath.Join("..", "game", "questions", "questions")

// TODO: does this need to be a cmap (does this need to be threadsafe?)
var categoryMap = map[string]string{} // map category to the file it is located in

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
			catFile.Close()
			fmt.Println("ioutil.ReadAll failed in questions.PopulateCategories")
			return
		}
		
		catFile.Close()
    
		data := QuestionsList{}
    
		err = json.Unmarshal(catFileBytes, &data)
		if err != nil {
			fmt.Println("json.Unmarshal failed in questions.PopulateCategories, err: ", err)
			return
		}
    
		for _, cat := range data.Categories {
			if len(cat.Questions) > 4 {
				categoryMap[cat.CategoryName] = filepath.Join(questionDir, f.Name())
				catList = append(catList, cat.CategoryName)
			}
		}
	}
	
	CategoriesInitialized = true
}

func GetGameCategories(gameId string) []string {

	// from the available categories, randomly select 6 to play with
	// TODO: kinda pointless when we only have six categories anyways
	// TODO (eventually): also randomly select questions (this function or somewhere else?)

	rand.Seed(time.Now().UnixNano())
	var gameCats []string
	p := rand.Perm(len(catList))
	for _, r := range(p[:6]) {
		gameCats = append(gameCats, catList[r])
	}

	cats := map[string][]*Question{}

	for _, cat := range(gameCats) {
		cats[cat] = getQuestionsForCategory(cat)
	}
	
	gameQuestions.Set(gameId, cats)
	
	return gameCats

} 

func getQuestionsForCategory(givenCategory string) []*Question {

	catFileName := categoryMap[givenCategory]
	catFile, err := os.Open(catFileName)
	if err != nil {
		fmt.Println("os.Open failed in questions.getQuestionsForCategory, filename = ", catFileName)
		return []*Question{}
	}
	
	catFileBytes, err := ioutil.ReadAll(catFile)
	if err != nil {
		catFile.Close()
		fmt.Println("ioutil.ReadAll failed in questions.PopulateCategories")
		return []*Question{}
	}
	
	catFile.Close()
	
	data := QuestionsList{}
    
	err = json.Unmarshal(catFileBytes, &data)
	if err != nil {
		fmt.Println("json.Unmarshal failed in questions.PopulateCategories, err: ", err)
		return []*Question{}
	}
	
	var selectedQuestions []*Question
	
	for _, cat := range(data.Categories) {
		if cat.CategoryName == givenCategory {
			p := rand.Perm(len(cat.Questions))
			for _, r := range(p[:5]) {
				selectedQuestions = append(selectedQuestions, cat.Questions[r])
			}
			break
		}
	}

	return selectedQuestions
}

func GetGameQuestion(gameId, category string, pointVal uint8) Question {

	if tmp, ok := gameQuestions.Get(gameId); ok {
		m := tmp.(map[string][]*Question)
		return *(m[category][(pointVal / 10)])
	}

	return Question{}
}


func RemoveGameQuestion(gameId, category string, pointVal uint8) {

	if tmp, ok := gameQuestions.Get(gameId); ok {
		m := tmp.(map[string][]*Question)
		
		m[category][(pointVal / 10)] = nil
		
		qFound := false
		for i := 0; i < 5; i++ {
			if m[category][i] != nil {
				qFound = true
				break
			}
		}
		
		if !qFound {
			delete(m, category)
		}
	} else {
		fmt.Println("question not removed")
	}
}







