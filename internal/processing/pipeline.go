package processing

import (
	"errors"
	"math"
)

// Step defines a pre/post-processing step
// Each step can be extended with more config fields as needed

type Step struct {
	Type   string                 `json:"type" mapstructure:"type"`
	Params map[string]interface{} `json:"params" mapstructure:"params"`
}

type Pipeline struct {
	Steps []Step
}

// NewPipelineFromConfig creates a Pipeline from a config slice
func NewPipelineFromConfig(cfg []map[string]interface{}) (*Pipeline, error) {
	steps := make([]Step, 0, len(cfg))
	for _, s := range cfg {
		typeVal, ok := s["type"].(string)
		if !ok {
			return nil, errors.New("pipeline step missing 'type'")
		}
		params := map[string]interface{}{}
		if p, ok := s["params"].(map[string]interface{}); ok {
			params = p
		}
		steps = append(steps, Step{Type: typeVal, Params: params})
	}
	return &Pipeline{Steps: steps}, nil
}

// Run executes the pipeline on the input data
func (p *Pipeline) Run(input interface{}) (interface{}, error) {
	var data = input
	for _, step := range p.Steps {
		switch step.Type {
		case "normalize":
			// Example: normalize float32 slice to [0,1]
			arr, ok := data.([]float32)
			if !ok {
				return nil, errors.New("normalize expects []float32 input")
			}
			var max float32
			for _, v := range arr {
				if v > max {
					max = v
				}
			}
			if max == 0 {
				return arr, nil
			}
			for i := range arr {
				arr[i] = arr[i] / max
			}
			data = arr
		case "softmax":
			arr, ok := data.([]float32)
			if !ok {
				return nil, errors.New("softmax expects []float32 input")
			}
			var max float32
			for _, v := range arr {
				if v > max {
					max = v
				}
			}
			var sum float32
			expArr := make([]float32, len(arr))
			for i, v := range arr {
				expArr[i] = float32(exp(float64(v - max)))
				sum += expArr[i]
			}
			for i := range expArr {
				expArr[i] /= sum
			}
			data = expArr
		case "apply_threshold":
			arr, ok := data.([]float32)
			if !ok {
				return nil, errors.New("apply_threshold expects []float32 input")
			}
			thresh, ok := step.Params["threshold"].(float64)
			if !ok {
				thresh = 0.5
			}
			binArr := make([]int, len(arr))
			for i, v := range arr {
				if float64(v) >= thresh {
					binArr[i] = 1
				} else {
					binArr[i] = 0
				}
			}
			data = binArr
		case "map_to_labels":
			arr, ok := data.([]int)
			if !ok {
				return nil, errors.New("map_to_labels expects []int input")
			}
			labelsIface, ok := step.Params["labels"].([]interface{})
			if !ok {
				return nil, errors.New("map_to_labels requires 'labels' param as []string")
			}
			labels := make([]string, len(labelsIface))
			for i, v := range labelsIface {
				labels[i], _ = v.(string)
			}
			mapped := make([]string, len(arr))
			for i, idx := range arr {
				if idx >= 0 && idx < len(labels) {
					mapped[i] = labels[idx]
				} else {
					mapped[i] = ""
				}
			}
			data = mapped
		// Add more built-in steps here
		default:
			return nil, errors.New("unsupported step: " + step.Type)
		}
	}
	return data, nil
}

// ProcessInput runs the pipeline as input pre-processing (alias for Run)
func (p *Pipeline) ProcessInput(input interface{}) (interface{}, error) {
	return p.Run(input)
}

// ProcessOutput runs the pipeline as output post-processing (alias for Run)
func (p *Pipeline) ProcessOutput(output interface{}) (interface{}, error) {
	return p.Run(output)
}

func exp(x float64) float64 {
	return math.Exp(x)
}
