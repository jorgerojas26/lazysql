package components

import (
	"context"
	"sync"
	"testing"
)

// performanceContractDriver adds count instrumentation to the existing
// deterministic refresh driver. The test below checks the navigation contract
// without relying on a wall-clock performance threshold.
type performanceContractDriver struct {
	*refreshCallDriver

	mu            sync.Mutex
	exactCalls    int
	estimateCalls int
}

func (driver *performanceContractDriver) GetEstimatedRowCount(context.Context, string, string) (*int64, error) {
	driver.mu.Lock()
	driver.estimateCalls++
	driver.mu.Unlock()
	return nil, nil
}

func (driver *performanceContractDriver) GetExactRowCount(context.Context, string, string, string) (int64, error) {
	driver.mu.Lock()
	driver.exactCalls++
	driver.mu.Unlock()
	return 0, nil
}

func (driver *performanceContractDriver) countCalls() (estimate, exact int) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.estimateCalls, driver.exactCalls
}

func TestPaginationUsesPageFetchOnlyAfterMetadataIsCached(t *testing.T) {
	driver := &performanceContractDriver{refreshCallDriver: newRefreshCallDriver()}
	table := newRefreshCallTable(driver)
	primeRefreshMetadata(t, table)

	// Automatic exact counting is disabled for this focused navigation seam.
	// The real menu remains installed, so the test still exercises the normal
	// page lifecycle while proving that navigation does not require a count.
	setCountConfig(t, 50_000, 0)
	stopApp := startRefreshApplication(t, table)
	defer stopApp()

	firstPage := make(chan struct{})
	table.FetchRecords(nil, func() { close(firstPage) })
	<-firstPage

	table.Pagination.SetOffset(table.Pagination.GetLimit())
	secondPage := make(chan struct{})
	table.FetchRecords(nil, func() { close(secondPage) })
	<-secondPage

	pageCalls, _, _, _, _ := driver.pageArgs()
	if pageCalls != 2 {
		t.Fatalf("pagination made %d page fetches, want exactly 2", pageCalls)
	}
	for _, kind := range metadataKinds {
		if got := driver.count(kind); got != 1 {
			t.Fatalf("pagination refetched %s %d times, want cached once", kind, got)
		}
	}
	estimateCalls, exactCalls := driver.countCalls()
	if estimateCalls > 1 || exactCalls != 0 {
		t.Fatalf("pagination count calls = estimate %d exact %d, want at most 1/0", estimateCalls, exactCalls)
	}
}
