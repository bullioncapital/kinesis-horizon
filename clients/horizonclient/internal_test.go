package horizonclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCurrentServerTime(t *testing.T) {
	t.Run("non-existing-host-name", func(t *testing.T) {
		currentTime := currentServerTime("non-existing-host-name", 60)
		require.Zerof(t, currentTime, "server time for non-existing time is expected to be zero, but was %d instead", currentTime)
	})

	t.Run("server-behind", func(t *testing.T) {
		serverTimeMapMutex.Lock()
		ServerTimeMap["TestCurrentServerTime-server-behind"] = ServerTimeRecord{ServerTime: 27, LocalTimeRecorded: 23}
		serverTimeMapMutex.Unlock()

		currentTime := currentServerTime("TestCurrentServerTime-server-behind", 500)
		defer func() {
			serverTimeMapMutex.Lock()
			delete(ServerTimeMap, "TestCurrentServerTime-server-behind")
			serverTimeMapMutex.Unlock()
		}()

		require.Zerof(t, currentTime, "server time is too old and the method should have returned 0; instead, %d was returned", currentTime)
	})

	t.Run("normal", func(t *testing.T) {
		serverTimeMapMutex.Lock()
		ServerTimeMap["TestCurrentServerTime-server"] = ServerTimeRecord{ServerTime: 27, LocalTimeRecorded: 23}
		serverTimeMapMutex.Unlock()

		defer func() {
			serverTimeMapMutex.Lock()
			delete(ServerTimeMap, "TestCurrentServerTime-server")
			serverTimeMapMutex.Unlock()
		}()

		currentTime := currentServerTime("TestCurrentServerTime-server", 37)
		require.Equalf(t, currentTime, int64(41), "currentServerTime should have returned %d, but returned %d instead", 41, currentTime)
	})
}
