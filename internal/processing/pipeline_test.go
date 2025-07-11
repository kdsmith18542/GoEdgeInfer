package processing

import "testing"

func TestPipeline_Run_Normalize(t *testing.T) {
	p := Pipeline{
		Steps: []Step{{Type: "normalize"}},
	}
	input := []float32{0, 2, 4}
	out, err := p.Run(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	arr := out.([]float32)
	if arr[1] != 0.5 || arr[2] != 1.0 {
		t.Errorf("normalize failed: got %v", arr)
	}
}

func TestPipeline_AllSteps(t *testing.T) {
	p, err := NewPipelineFromConfig([]map[string]interface{}{
		{"type": "normalize"},
		{"type": "softmax"},
		{"type": "apply_threshold", "params": map[string]interface{}{"threshold": 0.5}},
		{"type": "map_to_labels", "params": map[string]interface{}{"labels": []interface{}{"a", "b", "c"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ProcessInput([]float32{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.ProcessOutput([]float32{0.1, 0.2, 0.7})
	if err != nil {
		t.Fatal(err)
	}
}
