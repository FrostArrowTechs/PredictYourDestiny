package fortune

import "testing"

func TestAttachBirthMetadataDeclaresAssumptionsAndStableShape(t *testing.T) {
	hour := 12
	birth := BirthContext{Year: 2000, Month: 1, Day: 1, Hour: &hour, TimeZone: "Asia/Shanghai"}
	result := &Result{Kind: KindBazi, Data: map[string]string{"chart": "value"}}
	AttachBirthMetadata(result, Input{Birth: &birth})

	if result.ResultMetadata == nil {
		t.Fatal("result metadata was not attached")
	}
	meta := *result.ResultMetadata
	if meta.AlgorithmVersion == "" || meta.RuleSetVersion == "" || meta.InputPrecision != PrecisionHour {
		t.Fatalf("metadata = %+v", meta)
	}
	if len(meta.Assumptions) == 0 || len(meta.Warnings) == 0 || meta.StableFacts == nil || meta.VariableFacts == nil || meta.Variants == nil {
		t.Fatalf("metadata shape is incomplete: %+v", meta)
	}
	if meta.UnsupportedRules == nil {
		t.Fatalf("unsupportedRules must be a non-null array: %+v", meta)
	}
}

func TestAstrologyMetadataDeclaresUnsupportedPrecision(t *testing.T) {
	hour, minute := 12, 0
	birth := BirthContext{Year: 2000, Month: 1, Day: 1, Hour: &hour, Minute: &minute}
	result := &Result{Kind: KindAstrology, Data: &AstrologyResult{}}
	AttachBirthMetadata(result, Input{Birth: &birth})
	if result.ResultMetadata == nil || len(result.ResultMetadata.UnsupportedRules) == 0 {
		t.Fatalf("astrology limitations not declared: %+v", result.ResultMetadata)
	}
}
