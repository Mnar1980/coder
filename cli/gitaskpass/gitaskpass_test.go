package gitaskpass_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coder/coder/cli/gitaskpass"
)

func TestParse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in       string
		wantUser string
		wantHost string
	}{
		{
			in:       "Username for 'https://github.com': ",
			wantUser: "",
			wantHost: "https://github.com",
		},
		{
			in:       "Username for 'https://enterprise.github.com': ",
			wantUser: "",
			wantHost: "https://enterprise.github.com",
		},
		{
			in:       "Username for 'http://wow.io': ",
			wantUser: "",
			wantHost: "http://wow.io",
		},
		{
			in:       "Password for 'https://myuser@github.com': ",
			wantUser: "myuser",
			wantHost: "https://github.com",
		},
		{
			in:       "Password for 'https://myuser@enterprise.github.com': ",
			wantUser: "myuser",
			wantHost: "https://enterprise.github.com",
		},
		{
			in:       "Password for 'http://myuser@wow.io': ",
			wantUser: "myuser",
			wantHost: "http://wow.io",
		},
	} {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			user, host, err := gitaskpass.Parse(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.wantUser, user)
			require.Equal(t, tc.wantHost, host)
		})
	}
}
