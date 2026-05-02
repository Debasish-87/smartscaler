package utils_test

import (
	"testing"

	"smartscaler/pkg/utils"
)


func TestContains_Found(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !utils.Contains(list, "b") {
		t.Error("expected Contains to return true for existing element")
	}
}

func TestContains_NotFound(t *testing.T) {
	list := []string{"a", "b", "c"}
	if utils.Contains(list, "z") {
		t.Error("expected Contains to return false for missing element")
	}
}

func TestContains_EmptyList(t *testing.T) {
	if utils.Contains([]string{}, "a") {
		t.Error("expected Contains to return false on empty list")
	}
}

func TestContains_NilList(t *testing.T) {
	if utils.Contains(nil, "a") {
		t.Error("expected Contains to return false on nil list")
	}
}

func TestRemove_Existing(t *testing.T) {
	result := utils.Remove([]string{"a", "b", "c"}, "b")
	if utils.Contains(result, "b") {
		t.Error("expected 'b' to be removed")
	}
	if len(result) != 2 {
		t.Errorf("expected length 2, got %d", len(result))
	}
}

func TestRemove_NotExisting(t *testing.T) {
	original := []string{"a", "b", "c"}
	result := utils.Remove(original, "z")
	if len(result) != 3 {
		t.Errorf("remove of non-existing element should not change length, got %d", len(result))
	}
}

func TestRemove_EmptyList(t *testing.T) {
	result := utils.Remove([]string{}, "a")
	if len(result) != 0 {
		t.Errorf("remove on empty list should return empty, got %d", len(result))
	}
}

func TestRemove_DuplicatesRemovedAll(t *testing.T) {
	result := utils.Remove([]string{"a", "b", "a"}, "a")
	if utils.Contains(result, "a") {
		t.Error("expected all occurrences of 'a' to be removed")
	}
}

func TestAddUnique_NewElement(t *testing.T) {
	result := utils.AddUnique([]string{"a", "b"}, "c")
	if !utils.Contains(result, "c") {
		t.Error("expected 'c' to be added")
	}
	if len(result) != 3 {
		t.Errorf("expected length 3, got %d", len(result))
	}
}

func TestAddUnique_DuplicateNotAdded(t *testing.T) {
	result := utils.AddUnique([]string{"a", "b"}, "a")
	if len(result) != 2 {
		t.Errorf("duplicate should not be added, expected length 2, got %d", len(result))
	}
}

func TestAddUnique_EmptyList(t *testing.T) {
	result := utils.AddUnique([]string{}, "x")
	if len(result) != 1 || result[0] != "x" {
		t.Errorf("adding to empty list failed, got %v", result)
	}
}