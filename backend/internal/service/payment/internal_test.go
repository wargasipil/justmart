package payment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapPayoutStatus(t *testing.T) {
	require.Equal(t, StatusCompleted, mapPayoutStatus("SUCCEEDED"))
	require.Equal(t, StatusCompleted, mapPayoutStatus("completed"))
	require.Equal(t, StatusFailed, mapPayoutStatus("FAILED"))
	require.Equal(t, StatusFailed, mapPayoutStatus("CANCELLED"))
	require.Equal(t, StatusProcessing, mapPayoutStatus("ACCEPTED"))
	require.Equal(t, StatusProcessing, mapPayoutStatus("PENDING"))
	require.Equal(t, StatusProcessing, mapPayoutStatus("")) // unknown defaults to processing
}

func TestExternalID(t *testing.T) {
	require.Equal(t, "payslip_abc", externalID("payslip", "abc"))
}

func TestMaskKey(t *testing.T) {
	require.Equal(t, "", maskKey(""))
	require.Equal(t, "xnd_…1234", maskKey("xnd_development_ABCD1234"))
	require.NotContains(t, maskKey("xnd_development_ABCD1234"), "development")
}
