package fortune

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"testing"
)

type ziweiGoldenFixture struct {
	ID            string   `json:"id"`
	Source        string   `json:"source"`
	SourceVersion string   `json:"sourceVersion"`
	Verification  string   `json:"verification"`
	Reviewers     []string `json:"reviewers"`
	Input         struct {
		Year, Month, Day, Hour, Minute int
		Gender                         Gender
	} `json:"input"`
	Expected struct {
		LifePalaceBranch string              `json:"lifePalaceBranch"`
		BodyPalaceBranch string              `json:"bodyPalaceBranch"`
		LifeRuler        string              `json:"lifeRuler"`
		BodyRuler        string              `json:"bodyRuler"`
		WuXingJu         string              `json:"wuXingJu"`
		MainStarOfLife   string              `json:"mainStarOfLife"`
		StarsByBranch    map[string][]string `json:"starsByBranch"`
	} `json:"expected"`
}

func loadZiweiGoldenFixtures(t *testing.T) []ziweiGoldenFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/ziwei_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []ziweiGoldenFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestZiweiGoldenCharts(t *testing.T) {
	fixtures := loadZiweiGoldenFixtures(t)
	if len(fixtures) == 0 {
		t.Fatal("no independently sourced Ziwei golden charts")
	}
	ids := make(map[string]struct{}, len(fixtures))
	doubleHumanReviewed := 0
	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.ID == "" || fixture.Source == "" || fixture.SourceVersion == "" || fixture.Verification == "" {
				t.Fatal("golden fixture lacks provenance")
			}
			if _, exists := ids[fixture.ID]; exists {
				t.Fatalf("duplicate golden fixture id %q", fixture.ID)
			}
			ids[fixture.ID] = struct{}{}
			sourceURL, err := url.Parse(fixture.Source)
			if err != nil || sourceURL.Scheme != "https" || sourceURL.Host == "" {
				t.Fatalf("golden fixture source must be an absolute HTTPS URL: %q", fixture.Source)
			}
			if fixture.Verification == "double_human_verified" {
				uniqueReviewers := make(map[string]struct{}, len(fixture.Reviewers))
				for _, reviewer := range fixture.Reviewers {
					if reviewer != "" {
						uniqueReviewers[reviewer] = struct{}{}
					}
				}
				if len(uniqueReviewers) < 2 {
					t.Fatal("double_human_verified fixture requires two distinct reviewer identifiers")
				}
				doubleHumanReviewed++
			}
			if len(fixture.Expected.StarsByBranch) != 12 {
				t.Fatalf("full-chart fixture has %d branches, want 12", len(fixture.Expected.StarsByBranch))
			}
			result, err := (ZiweiEngine{}).Compute(Input{Year: fixture.Input.Year, Month: fixture.Input.Month, Day: fixture.Input.Day, Hour: fixture.Input.Hour, Minute: fixture.Input.Minute, Gender: fixture.Input.Gender})
			if err != nil {
				t.Fatal(err)
			}
			chart := result.Data.(*ZiweiChart)
			if chart.LifePalaceBranch != fixture.Expected.LifePalaceBranch || chart.BodyPalaceBranch != fixture.Expected.BodyPalaceBranch || chart.LifeRuler != fixture.Expected.LifeRuler || chart.BodyRuler != fixture.Expected.BodyRuler || chart.WuXingJu != fixture.Expected.WuXingJu || chart.MainStarOfLife != fixture.Expected.MainStarOfLife {
				lifeStars := []string{}
				for _, palace := range chart.Palaces {
					if palace.IsLife {
						lifeStars = palace.Stars
					}
				}
				t.Fatalf("core chart differs: got life/body=%s/%s rulers=%s/%s ju=%s main=%s lifeStars=%v", chart.LifePalaceBranch, chart.BodyPalaceBranch, chart.LifeRuler, chart.BodyRuler, chart.WuXingJu, chart.MainStarOfLife, lifeStars)
			}
			actual := map[string][]string{}
			for _, palace := range chart.Palaces {
				actual[palace.Branch] = append([]string{}, palace.Stars...)
				sort.Strings(actual[palace.Branch])
			}
			for branch, stars := range fixture.Expected.StarsByBranch {
				sort.Strings(stars)
				if !equalStrings(actual[branch], stars) {
					t.Fatalf("%s stars got %v want %v", branch, actual[branch], stars)
				}
			}
		})
	}
	t.Logf("Ziwei golden corpus: technical=%d/30, double-human-reviewed=%d/30", len(fixtures), doubleHumanReviewed)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
