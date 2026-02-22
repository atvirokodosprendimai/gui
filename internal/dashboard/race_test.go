package dashboard

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWatcherFanout_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	a := New(nil, 0, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					ch := a.addWatcher()
					a.fanoutUpdate(uiUpdate{Element: "overview", Scope: scopeGlobal})
					a.removeWatcher(ch)
				}
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					a.fanoutUpdate(uiUpdate{Element: "records", Scope: scopeGlobal})
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
}

func TestLoginLimiter_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	a := New(nil, 0, false)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("acct:user@example.com|ip:203.0.113.%d", id%4)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					a.noteLoginFail(key)
					_ = a.allowLogin(key)
					a.noteLoginSuccess(key)
				}
			}
		}(i)
	}

	<-ctx.Done()
	wg.Wait()
}
