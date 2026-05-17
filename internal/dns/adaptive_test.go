package dns

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdaptiveSemaphoreNoDeadlock simula o pior caso: SetLimit
// reduzindo bruscamente o limite enquanto várias goroutines fazem
// Acquire/Release. Não deve travar.
func TestAdaptiveSemaphoreNoDeadlock(t *testing.T) {
	sem := NewAdaptiveSemaphore(100, 4096)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var acquired atomic.Int64

	// 200 workers concorrentes fazendo Acquire/Release em loop
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if err := sem.Acquire(ctx); err != nil {
					return
				}
				acquired.Add(1)
				time.Sleep(time.Microsecond * 10)
				sem.Release()
			}
		}()
	}

	// SetLimit oscilando agressivamente — testa o bug do deadlock
	wg.Add(1)
	go func() {
		defer wg.Done()
		limits := []int{50, 200, 10, 300, 5, 500, 100}
		for _, l := range limits {
			sem.SetLimit(l)
			time.Sleep(time.Millisecond * 50)
			if ctx.Err() != nil {
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
		t.Logf("OK — %d Acquire/Release sem deadlock", acquired.Load())
	case <-time.After(8 * time.Second):
		t.Fatalf("DEADLOCK — apenas %d acquires concluídos antes do timeout", acquired.Load())
	}

	// Sanity: inflight deve ser 0 ao fim
	if inf := sem.Inflight(); inf != 0 {
		t.Errorf("inflight devia ser 0 ao fim, foi %d", inf)
	}
}

// TestAdaptiveSemaphoreRespectsLimit verifica que o semáforo não
// passa do limit configurado simultaneamente.
func TestAdaptiveSemaphoreRespectsLimit(t *testing.T) {
	const limit = 32
	sem := NewAdaptiveSemaphore(limit, 1000)

	var maxSeen atomic.Int64
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx); err != nil {
				return
			}
			defer sem.Release()
			cur := sem.Inflight()
			for {
				old := maxSeen.Load()
				if cur <= old || maxSeen.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
		}()
	}
	wg.Wait()

	if maxSeen.Load() > int64(limit) {
		t.Errorf("inflight passou do limit %d: viu %d", limit, maxSeen.Load())
	}
	t.Logf("OK — max inflight observado: %d (limit=%d)", maxSeen.Load(), limit)
}

// TestAdaptiveSemaphoreContextCancel verifica que Acquire respeita ctx.Done.
func TestAdaptiveSemaphoreContextCancel(t *testing.T) {
	sem := NewAdaptiveSemaphore(1, 100)
	// ocupa o único slot
	if err := sem.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer sem.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := sem.Acquire(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Acquire devia ter falhado, retornou nil em %v", elapsed)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Acquire demorou %v — devia retornar logo após ctx expirar", elapsed)
	}
}

// TestAIMDIncrease simula erro baixo e verifica que limit sobe.
func TestAIMDIncrease(t *testing.T) {
	sem := NewAdaptiveSemaphore(64, 4096)

	// Mock: sempre retorna 100 total, 0 erros = error_rate = 0
	getRate := func() (uint64, uint64) { return 100, 0 }

	ctrl := NewAIMDController(sem, getRate, noopLogger())
	ctrl.tickInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ctrl.Run(ctx)

	if limit := sem.Limit(); limit <= 64 {
		t.Errorf("limit não subiu com erro 0%%, ficou em %d", limit)
	}
}

// TestAIMDFastDecrease simula erro crítico e verifica decrease agressivo.
func TestAIMDFastDecrease(t *testing.T) {
	sem := NewAdaptiveSemaphore(1024, 4096)

	// Mock: sempre 100 total, 50 erros = 50% erro
	getRate := func() (uint64, uint64) { return 100, 50 }

	ctrl := NewAIMDController(sem, getRate, noopLogger())
	ctrl.tickInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	ctrl.Run(ctx)

	// Com erro 50% e fastFactor 4, em 100ms (10 ticks) deveria cair muito.
	if limit := sem.Limit(); limit >= 1024 {
		t.Errorf("limit não caiu com erro 50%%, ficou em %d", limit)
	}
}
