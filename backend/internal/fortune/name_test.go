package fortune

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

func nameTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CharacterStroke{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNameEngineRejectsUnknownCharacters(t *testing.T) {
	db := nameTestDB(t)
	if err := db.Create(&model.CharacterStroke{Char: "张", Strokes: 11}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := (NameEngine{DB: db}).Compute(Input{Question: "张罕"})
	var unknown *UnknownCharactersError
	if !errors.As(err, &unknown) || len(unknown.Characters) != 1 || unknown.Characters[0] != "罕" {
		t.Fatalf("expected unknown-character error for 罕, got %v", err)
	}
}

func TestNameEngineDoesNotEstimateWithoutDictionary(t *testing.T) {
	_, err := (NameEngine{}).Compute(Input{Question: "张三"})
	if !errors.Is(err, ErrStrokeDictionaryUnavailable) {
		t.Fatalf("expected dictionary unavailable, got %v", err)
	}
}

func TestSplitNameRecognizesThreeCharacterCompoundSurname(t *testing.T) {
	surname, given := (NameEngine{}).splitName([]rune("欧阳修"))
	if string(surname) != "欧阳" || string(given) != "修" {
		t.Fatalf("split 欧阳修 into %q / %q", string(surname), string(given))
	}
}

func TestNameEngineStructuredInputAndVersionedDimensions(t *testing.T) {
	db := nameTestDB(t)
	for _, row := range []model.CharacterStroke{{Char: "欧", Strokes: 15}, {Char: "阳", Strokes: 17}, {Char: "修", Strokes: 10}} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := (NameEngine{DB: db}).Compute(Input{Surname: "欧阳", GivenName: "修", SurnameConfirmed: true, Script: "zh-Hans", StrokeStandard: "kangxi"})
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*NameResult)
	if chart.Surname != "欧阳" || chart.GivenName != "修" || !chart.SurnameConfirmed || chart.InputMode != "structured" {
		t.Fatalf("structured identity lost: %+v", chart)
	}
	if chart.DictionaryVersion != NameDictionaryVersion || chart.StrokeStandard != NameStrokeStandard || len(chart.Evaluations) != 4 {
		t.Fatalf("versioned dimensions incomplete: %+v", chart)
	}
	if chart.Evaluations[1].Score != nil || chart.Evaluations[1].Status != "unavailable" {
		t.Fatalf("unavailable pronunciation received a fake score: %+v", chart.Evaluations[1])
	}
}

func TestNameEngineRequiresSurnameConfirmation(t *testing.T) {
	_, err := (NameEngine{DB: nameTestDB(t)}).Compute(Input{Surname: "欧阳", GivenName: "修"})
	if err == nil {
		t.Fatal("unconfirmed structured surname was accepted")
	}
}
