package youtube

import "testing"

func TestDesktopPlayabilityStatusInferLoginRequiredError(t *testing.T) {
	tests := []struct {
		name   string
		status DesktopPlayabilityStatus
		want   bool
	}{
		{
			name: "login required with unicode apostrophe bot check reason",
			status: DesktopPlayabilityStatus{
				Status: "LOGIN_REQUIRED",
				Reason: "Sign in to confirm you’re not a bot",
			},
			want: true,
		},
		{
			name: "login required with ascii apostrophe bot check reason",
			status: DesktopPlayabilityStatus{
				Status: "LOGIN_REQUIRED",
				Reason: "Sign in to confirm you're not a bot",
			},
			want: true,
		},
		{
			name: "login required with bot check in messages",
			status: DesktopPlayabilityStatus{
				Status:   "LOGIN_REQUIRED",
				Messages: []string{"Sign in to confirm you are not a bot"},
			},
			want: true,
		},
		{
			name: "login required with unrelated reason",
			status: DesktopPlayabilityStatus{
				Status: "LOGIN_REQUIRED",
				Reason: "This content is unavailable",
			},
			want: false,
		},
		{
			name: "non login required status",
			status: DesktopPlayabilityStatus{
				Status: "OK",
				Reason: "Sign in to confirm you're not a bot",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.inferLoginRequiredError()
			if got != tt.want {
				t.Fatalf("inferLoginRequiredError()=%v, want %v", got, tt.want)
			}
		})
	}
}
