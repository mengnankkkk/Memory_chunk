package server

import (
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/engine"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapFragmentType(fragmentType refinerv1.FragmentType) engine.FragmentType {
	switch fragmentType {
	case refinerv1.FragmentType_FRAGMENT_TYPE_TITLE:
		return engine.FragmentTypeTitle
	case refinerv1.FragmentType_FRAGMENT_TYPE_CODE:
		return engine.FragmentTypeCode
	case refinerv1.FragmentType_FRAGMENT_TYPE_TABLE:
		return engine.FragmentTypeTable
	case refinerv1.FragmentType_FRAGMENT_TYPE_JSON:
		return engine.FragmentTypeJSON
	case refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT:
		return engine.FragmentTypeToolOutput
	case refinerv1.FragmentType_FRAGMENT_TYPE_LOG:
		return engine.FragmentTypeLog
	case refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK:
		return engine.FragmentTypeErrorStack
	default:
		return engine.FragmentTypeBody
	}
}
