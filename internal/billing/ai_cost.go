package billing

import (
	"fmt"
	"math"
	"strings"
)

const (
	defaultUSDToVND              = 25350.0
	audioTokensPerSecond         = 32.0
	gemini31LivePricingVersion   = "google-gemini-3.1-flash-live-2026-04"
	gemini3PricingVersion        = "google-gemini-3-flash-2026-04"
	gemini25NativePricingVersion = "google-gemini-2.5-native-audio-2026-04"
)

type aiCostRates struct {
	Version               string
	InputTextPerMillion   float64
	InputAudioPerMillion  float64
	InputVideoPerMillion  float64
	OutputTextPerMillion  float64
	OutputAudioPerMillion float64
	InputAudioPerMinute   float64
	InputVideoPerMinute   float64
	OutputAudioPerMinute  float64
}

type aiTokenBreakdown struct {
	InputText   float64
	InputAudio  float64
	InputVideo  float64
	OutputText  float64
	OutputAudio float64
}

func priceAICostEvent(input AICostEventInput) AICostEventInput {
	if input.CostUSD > 0 {
		input.CostUSD = roundCost(input.CostUSD)
		input.CostVND = roundQuantity(input.CostVND)
		return input
	}

	rates := ratesForAIModel(input.Provider, input.Model)
	breakdown, estimated := tokenBreakdownForCost(input)
	costUSD := costFromBreakdown(breakdown, rates)
	if estimated {
		costUSD += estimatedMediaCostFromSeconds(input, breakdown, rates)
	}
	if costUSD == 0 {
		return input
	}

	input.CostUSD = roundCost(costUSD)
	if input.CostVND == 0 {
		input.CostVND = roundQuantity(input.CostUSD * defaultUSDToVND)
	} else {
		input.CostVND = roundQuantity(input.CostVND)
	}
	input.Estimated = input.Estimated || estimated
	if strings.TrimSpace(input.PricingVersion) == "" {
		input.PricingVersion = rates.Version
	}
	return input
}

func ratesForAIModel(provider string, model string) aiCostRates {
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	if normalizedProvider != "google" {
		return aiCostRates{Version: "unknown-provider"}
	}
	if strings.Contains(normalizedModel, "gemini-3.1-flash-live") {
		return aiCostRates{
			Version:               gemini31LivePricingVersion,
			InputTextPerMillion:   0.75,
			InputAudioPerMillion:  3.00,
			InputVideoPerMillion:  1.00,
			OutputTextPerMillion:  4.50,
			OutputAudioPerMillion: 12.00,
			InputAudioPerMinute:   0.005,
			InputVideoPerMinute:   0.002,
			OutputAudioPerMinute:  0.018,
		}
	}
	if strings.Contains(normalizedModel, "native-audio") {
		return aiCostRates{
			Version:               gemini25NativePricingVersion,
			InputTextPerMillion:   0.50,
			InputAudioPerMillion:  3.00,
			InputVideoPerMillion:  3.00,
			OutputTextPerMillion:  2.00,
			OutputAudioPerMillion: 12.00,
		}
	}
	if strings.Contains(normalizedModel, "gemini-3") ||
		strings.Contains(normalizedModel, "gemini-3.1") {
		return aiCostRates{
			Version:               gemini3PricingVersion,
			InputTextPerMillion:   0.50,
			InputAudioPerMillion:  1.00,
			InputVideoPerMillion:  0.50,
			OutputTextPerMillion:  3.00,
			OutputAudioPerMillion: 3.00,
		}
	}
	return aiCostRates{
		Version:               "google-gemini-2.5-flash-2026-04",
		InputTextPerMillion:   0.30,
		InputAudioPerMillion:  1.00,
		InputVideoPerMillion:  0.30,
		OutputTextPerMillion:  2.50,
		OutputAudioPerMillion: 2.50,
	}
}

func tokenBreakdownForCost(input AICostEventInput) (aiTokenBreakdown, bool) {
	breakdown := aiTokenBreakdown{}
	estimated := false
	addUsageMetadataBreakdown(&breakdown, input.Metadata)

	if breakdown.InputText+breakdown.InputAudio+breakdown.InputVideo == 0 &&
		input.InputTokens > 0 {
		if strings.Contains(strings.ToLower(input.Modality), "audio") {
			breakdown.InputAudio = float64(input.InputTokens)
		} else {
			breakdown.InputText = float64(input.InputTokens)
		}
	}
	if breakdown.OutputText+breakdown.OutputAudio == 0 && input.OutputTokens > 0 {
		if strings.Contains(strings.ToLower(input.Modality), "audio") {
			breakdown.OutputAudio = float64(input.OutputTokens)
		} else {
			breakdown.OutputText = float64(input.OutputTokens)
		}
	}
	if breakdown.InputAudio == 0 && input.AudioInputSeconds > 0 {
		estimated = true
	}
	if breakdown.OutputAudio == 0 && input.AudioOutputSeconds > 0 {
		estimated = true
	}
	if breakdown.InputVideo == 0 && input.VideoSeconds > 0 {
		estimated = true
	}

	return breakdown, estimated
}

func estimatedMediaCostFromSeconds(
	input AICostEventInput,
	breakdown aiTokenBreakdown,
	rates aiCostRates,
) float64 {
	cost := 0.0
	if breakdown.InputAudio == 0 && input.AudioInputSeconds > 0 {
		if rates.InputAudioPerMinute > 0 {
			cost += input.AudioInputSeconds / 60 * rates.InputAudioPerMinute
		} else {
			cost += input.AudioInputSeconds * audioTokensPerSecond / 1_000_000 *
				rates.InputAudioPerMillion
		}
	}
	if breakdown.OutputAudio == 0 && input.AudioOutputSeconds > 0 {
		if rates.OutputAudioPerMinute > 0 {
			cost += input.AudioOutputSeconds / 60 * rates.OutputAudioPerMinute
		} else {
			cost += input.AudioOutputSeconds * audioTokensPerSecond / 1_000_000 *
				rates.OutputAudioPerMillion
		}
	}
	if breakdown.InputVideo == 0 && input.VideoSeconds > 0 {
		if rates.InputVideoPerMinute > 0 {
			cost += input.VideoSeconds / 60 * rates.InputVideoPerMinute
		} else {
			cost += input.VideoSeconds * audioTokensPerSecond / 1_000_000 *
				rates.InputVideoPerMillion
		}
	}
	return cost
}

func costFromBreakdown(breakdown aiTokenBreakdown, rates aiCostRates) float64 {
	return (breakdown.InputText*rates.InputTextPerMillion +
		breakdown.InputAudio*rates.InputAudioPerMillion +
		breakdown.InputVideo*rates.InputVideoPerMillion +
		breakdown.OutputText*rates.OutputTextPerMillion +
		breakdown.OutputAudio*rates.OutputAudioPerMillion) / 1_000_000
}

func addUsageMetadataBreakdown(
	breakdown *aiTokenBreakdown,
	metadata map[string]any,
) {
	rawEvents, ok := metadata["raw_usage_metadata"].([]any)
	if !ok {
		addOneUsageMetadataBreakdown(breakdown, metadata)
		return
	}
	for _, rawEvent := range rawEvents {
		event, ok := rawEvent.(map[string]any)
		if !ok {
			continue
		}
		addOneUsageMetadataBreakdown(breakdown, event)
	}
}

func addOneUsageMetadataBreakdown(
	breakdown *aiTokenBreakdown,
	metadata map[string]any,
) {
	addTokenDetails(
		breakdown,
		metadata,
		true,
		"prompt_tokens_details",
		"prompt_token_details",
		"input_tokens_details",
		"input_token_details",
	)
	addTokenDetails(
		breakdown,
		metadata,
		false,
		"candidates_tokens_details",
		"candidates_token_details",
		"output_tokens_details",
		"output_token_details",
	)
	addNamedTokenCounters(breakdown, metadata)
}

func addTokenDetails(
	breakdown *aiTokenBreakdown,
	metadata map[string]any,
	input bool,
	keys ...string,
) {
	for _, key := range keys {
		details, ok := metadata[key].([]any)
		if !ok {
			continue
		}
		for _, rawDetail := range details {
			detail, ok := rawDetail.(map[string]any)
			if !ok {
				continue
			}
			modality := strings.ToLower(fmt.Sprint(detail["modality"]))
			tokens := numberFromAny(detail["token_count"])
			if tokens == 0 {
				tokens = numberFromAny(detail["tokens"])
			}
			addTokensByModality(breakdown, modality, tokens, input)
		}
	}
}

func addNamedTokenCounters(
	breakdown *aiTokenBreakdown,
	metadata map[string]any,
) {
	breakdown.InputText += numberFromAny(metadata["input_text_tokens"])
	breakdown.InputAudio += numberFromAny(metadata["input_audio_tokens"])
	breakdown.InputVideo += numberFromAny(metadata["input_video_tokens"])
	breakdown.OutputText += numberFromAny(metadata["output_text_tokens"])
	breakdown.OutputAudio += numberFromAny(metadata["output_audio_tokens"])
}

func addTokensByModality(
	breakdown *aiTokenBreakdown,
	modality string,
	tokens float64,
	input bool,
) {
	if tokens <= 0 {
		return
	}
	switch {
	case strings.Contains(modality, "audio"):
		if input {
			breakdown.InputAudio += tokens
		} else {
			breakdown.OutputAudio += tokens
		}
	case strings.Contains(modality, "video"):
		breakdown.InputVideo += tokens
	default:
		if input {
			breakdown.InputText += tokens
		} else {
			breakdown.OutputText += tokens
		}
	}
}

func numberFromAny(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0
		}
		return f
	default:
		return 0
	}
}
