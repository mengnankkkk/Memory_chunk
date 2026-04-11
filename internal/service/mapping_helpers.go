package service

import (
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/core"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mapFragmentType(fragmentType refinerv1.FragmentType) core.FragmentType {
	switch fragmentType {
	case refinerv1.FragmentType_FRAGMENT_TYPE_TITLE:
		return core.FragmentTypeTitle
	case refinerv1.FragmentType_FRAGMENT_TYPE_CODE:
		return core.FragmentTypeCode
	case refinerv1.FragmentType_FRAGMENT_TYPE_TABLE:
		return core.FragmentTypeTable
	case refinerv1.FragmentType_FRAGMENT_TYPE_JSON:
		return core.FragmentTypeJSON
	case refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT:
		return core.FragmentTypeToolOutput
	case refinerv1.FragmentType_FRAGMENT_TYPE_LOG:
		return core.FragmentTypeLog
	case refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK:
		return core.FragmentTypeErrorStack
	default:
		return core.FragmentTypeBody
	}
}
