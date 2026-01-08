package models

import (
	"testing"
)

func TestRequiredModels(t *testing.T) {
	models := RequiredModels()

	if len(models) != 29 {
		t.Errorf("expected 29 required models, got %d", len(models))
	}

	// Check that all models have valid data
	for _, model := range models {
		if model.Name == "" {
			t.Error("model has empty name")
		}
		if model.URL == "" {
			t.Error("model has empty URL")
		}
		if model.Size <= 0 {
			t.Errorf("model %s has invalid size %d", model.Name, model.Size)
		}
	}

	// Verify specific models exist
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}

	expectedModels := []string{
		"wan2.2_i2v_high_noise_14B_fp16.safetensors",
		"wan2.2_i2v_low_noise_14B_fp16.safetensors",
		"umt5_xxl_fp16.safetensors",
		"wan_2.1_vae.safetensors",
		"wan2.2_lightning_high_noise.safetensors",
		"wan2.2_lightning_low_noise.safetensors",
		"qwen_image_edit_2511_bf16.safetensors",
		"qwen_2.5_vl_7b.safetensors",
		"qwen_image_vae.safetensors",
		"Qwen-Image-Edit-2511-Lightning-4steps-V1.0-bf16.safetensors",
		"qwen_tokenizer/tokenizer.json",
		"qwen_tokenizer/vocab.json",
		"qwen_tokenizer/merges.txt",
		"qwen_tokenizer/tokenizer_config.json",
	}

	for _, expected := range expectedModels {
		if !modelNames[expected] {
			t.Errorf("expected model %s not found in RequiredModels()", expected)
		}
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1000", 1000},
		{"0", 0},
		{"", 0},
		{"abc", 0},
		{"123456789", 123456789},
	}

	for _, tt := range tests {
		result := parseSize(tt.input)
		if result != tt.expected {
			t.Errorf("parseSize(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestDownloaderNew(t *testing.T) {
	// Create downloader with nil client (for testing)
	downloader := NewDownloader(nil, "/models", "test_token")

	if downloader.modelsDir != "/models" {
		t.Errorf("expected modelsDir /models, got %s", downloader.modelsDir)
	}

	if downloader.hfToken != "test_token" {
		t.Errorf("expected hfToken test_token, got %s", downloader.hfToken)
	}
}

func TestModelFileURL(t *testing.T) {
	models := RequiredModels()
	hfBase := "https://huggingface.co/"
	validPrefixes := []string{
		"https://huggingface.co/Comfy-Org/",
		"https://huggingface.co/lightx2v/",
		"https://huggingface.co/Qwen/",  // For tokenizer files
		"https://huggingface.co/dphn/",  // For Dolphin-Mistral chat model
	}

	// Verify all URLs are valid HuggingFace URLs
	for _, model := range models {
		if len(model.URL) == 0 {
			t.Errorf("model %s has empty URL", model.Name)
			continue
		}

		// URLs should be HuggingFace URLs
		if len(model.URL) < len(hfBase) || model.URL[:len(hfBase)] != hfBase {
			t.Errorf("model %s URL should start with %s, got %s", model.Name, hfBase, model.URL)
		}

		// Check if URL has one of the valid prefixes
		hasValidPrefix := false
		for _, prefix := range validPrefixes {
			if len(model.URL) >= len(prefix) && model.URL[:len(prefix)] == prefix {
				hasValidPrefix = true
				break
			}
		}
		if !hasValidPrefix {
			t.Errorf("model %s URL should start with one of %v, got %s", model.Name, validPrefixes, model.URL)
		}

		// Model weight files should end with .safetensors, config/tokenizer files have other extensions
		isTokenizerOrConfigFile := len(model.Name) >= 14 && model.Name[:14] == "qwen_tokenizer" ||
			model.Name == "dolphin-mistral-24b/config.json" ||
			model.Name == "dolphin-mistral-24b/model.safetensors.index.json" ||
			model.Name == "dolphin-mistral-24b/tokenizer.json" ||
			model.Name == "dolphin-mistral-24b/tokenizer_config.json" ||
			model.Name == "dolphin-mistral-24b/special_tokens_map.json"

		if !isTokenizerOrConfigFile {
			if len(model.URL) < 12 || model.URL[len(model.URL)-12:] != ".safetensors" {
				t.Errorf("model %s URL should end with .safetensors, got %s", model.Name, model.URL)
			}
		}
	}
}
