package mapper

import (
	"strings"

	refinerv1 "context-refiner/api/refinerv1"
	"context-refiner/internal/domain/core"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeMessageRole(value string) string {
	role := strings.TrimSpace(strings.ToLower(value))
	if role == "" {
		return "user"
	}
	return role
}

func mapFragmentTypeFromProto(fragmentType refinerv1.FragmentType) string {
	switch fragmentType {
	case refinerv1.FragmentType_FRAGMENT_TYPE_TITLE:
		return string(core.FragmentTypeTitle)
	case refinerv1.FragmentType_FRAGMENT_TYPE_CODE:
		return string(core.FragmentTypeCode)
	case refinerv1.FragmentType_FRAGMENT_TYPE_TABLE:
		return string(core.FragmentTypeTable)
	case refinerv1.FragmentType_FRAGMENT_TYPE_JSON:
		return string(core.FragmentTypeJSON)
	case refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT:
		return string(core.FragmentTypeToolOutput)
	case refinerv1.FragmentType_FRAGMENT_TYPE_LOG:
		return string(core.FragmentTypeLog)
	case refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK:
		return string(core.FragmentTypeErrorStack)
	default:
		return string(core.FragmentTypeBody)
	}
}

func mapFragmentTypeToCore(fragmentType string) core.FragmentType {
	switch strings.TrimSpace(fragmentType) {
	case string(core.FragmentTypeTitle):
		return core.FragmentTypeTitle
	case string(core.FragmentTypeCode):
		return core.FragmentTypeCode
	case string(core.FragmentTypeTable):
		return core.FragmentTypeTable
	case string(core.FragmentTypeJSON):
		return core.FragmentTypeJSON
	case string(core.FragmentTypeToolOutput):
		return core.FragmentTypeToolOutput
	case string(core.FragmentTypeLog):
		return core.FragmentTypeLog
	case string(core.FragmentTypeErrorStack):
		return core.FragmentTypeErrorStack
	default:
		return core.FragmentTypeBody
	}
}

func mapFragmentTypeToProto(fragmentType string) refinerv1.FragmentType {
	switch mapFragmentTypeToCore(fragmentType) {
	case core.FragmentTypeTitle:
		return refinerv1.FragmentType_FRAGMENT_TYPE_TITLE
	case core.FragmentTypeCode:
		return refinerv1.FragmentType_FRAGMENT_TYPE_CODE
	case core.FragmentTypeTable:
		return refinerv1.FragmentType_FRAGMENT_TYPE_TABLE
	case core.FragmentTypeJSON:
		return refinerv1.FragmentType_FRAGMENT_TYPE_JSON
	case core.FragmentTypeToolOutput:
		return refinerv1.FragmentType_FRAGMENT_TYPE_TOOL_OUTPUT
	case core.FragmentTypeLog:
		return refinerv1.FragmentType_FRAGMENT_TYPE_LOG
	case core.FragmentTypeErrorStack:
		return refinerv1.FragmentType_FRAGMENT_TYPE_ERROR_STACK
	default:
		return refinerv1.FragmentType_FRAGMENT_TYPE_BODY
	}
}
