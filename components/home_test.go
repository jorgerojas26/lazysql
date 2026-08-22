package components

import "testing"

func TestClampTreeWidth(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		containerWidth int
		want           int
	}{
		{
			name:           "width within bounds is unchanged",
			width:          30,
			containerWidth: 120,
			want:           30,
		},
		{
			name:           "does not exceed half the container",
			width:          80,
			containerWidth: 100,
			want:           50,
		},
		{
			name:           "floors at min width",
			width:          10,
			containerWidth: 120,
			want:           minTreeWidth,
		},
		{
			name:           "unknown container keeps requested width above min",
			width:          30,
			containerWidth: 0,
			want:           30,
		},
		{
			name:           "unknown container still floors at min",
			width:          5,
			containerWidth: 0,
			want:           minTreeWidth,
		},
		{
			name:           "tiny container does not overflow",
			width:          30,
			containerWidth: 20,
			want:           20,
		},
		{
			name:           "min width wins over half in small containers",
			width:          30,
			containerWidth: 40,
			want:           minTreeWidth,
		},
		{
			name:           "exactly half is allowed",
			width:          50,
			containerWidth: 100,
			want:           50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampTreeWidth(tt.width, tt.containerWidth)
			if got != tt.want {
				t.Errorf("clampTreeWidth(%d, %d) = %d, want %d", tt.width, tt.containerWidth, got, tt.want)
			}
		})
	}
}
