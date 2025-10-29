// main.go
package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// ------------------------- Helpers -------------------------
func nowMs() int64 { return time.Now().UnixNano() / int64(time.Millisecond) }

func writeCSVHeader(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := []string{
		"run_id", "mode", "trace", "chosen_branch",
		"time_trace_ms", "time_A_ms", "time_B_ms", "total_ms", "extra",
	}
	return w.Write(header)
}

func appendCSV(path string, row []string) error {
	// FIX: flags combinados con |
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	return w.Write(row)
}

func ms(d time.Duration) string { return strconv.FormatInt(int64(d/time.Millisecond), 10) }

// ------------------------- Funciones intensivas (con cancelación) -------------------------

// SimularProofOfWork: acepta ctx para cancelación.
// Retorna hash (o "" si cancelado), nonce (-1 si cancelado), y duración.
func SimularProofOfWork(ctx context.Context, data string, dificultad int) (string, int, time.Duration) {
	start := time.Now()
	targetPrefix := strings.Repeat("0", dificultad)
	nonce := 0
	for {
		select {
		case <-ctx.Done():
			return "", -1, time.Since(start)
		default:
			testData := fmt.Sprintf("%s%d", data, nonce)
			hash := sha256.Sum256([]byte(testData))
			hashStr := fmt.Sprintf("%x", hash)
			if strings.HasPrefix(hashStr, targetPrefix) {
				return hashStr, nonce, time.Since(start)
			}
			nonce++
			// punto de preempción ligero
			if nonce%100000 == 0 {
				select {
				case <-ctx.Done():
					return "", -1, time.Since(start)
				default:
				}
			}
		}
	}
}

// EncontrarPrimos: acepta ctx para cancelación.
// Retorna slice de primos (parcial si se canceló) y duración.
func EncontrarPrimos(ctx context.Context, max int) ([]int, time.Duration) {
	start := time.Now()
	primes := []int{}
	for i := 2; i <= max; i++ {
		select {
		case <-ctx.Done():
			return primes, time.Since(start)
		default:
			isPrime := true
			for j := 2; j*j <= i; j++ {
				if i%j == 0 {
					isPrime = false
					break
				}
			}
			if isPrime {
				primes = append(primes, i)
			}
		}
		// chequeo de cancel periódico
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return primes, time.Since(start)
			default:
			}
		}
	}
	return primes, time.Since(start)
}

// CalcularTrazaDeProductoDeMatrices: acepta ctx para cancelación.
// Retorna traza parcial (si se canceló) y duración.
func CalcularTrazaDeProductoDeMatrices(ctx context.Context, n int) (int, time.Duration) {
	start := time.Now()
	// crear matrices
	m1 := make([][]int, n)
	m2 := make([][]int, n)
	for i := 0; i < n; i++ {
		m1[i] = make([]int, n)
		m2[i] = make([]int, n)
		for j := 0; j < n; j++ {
			m1[i][j] = rand.Intn(10)
			m2[i][j] = rand.Intn(10)
		}
	}
	// multiplicación y traza
	trace := 0
	for i := 0; i < n; i++ {
		select {
		case <-ctx.Done():
			return trace, time.Since(start)
		default:
			sum := 0
			for k := 0; k < n; k++ {
				sum += m1[i][k] * m2[k][i]
			}
			trace += sum
		}
		// periodic cancel check
		if i%10 == 0 {
			select {
			case <-ctx.Done():
				return trace, time.Since(start)
			default:
			}
		}
	}
	return trace, time.Since(start)
}

// ------------------------- Modos de ejecución -------------------------

type resA struct {
	hash  string
	nonce int
	dur   time.Duration
}
type resB struct {
	primesCount int
	dur         time.Duration
}

// ejecucionEspeculativa: lanza A y B en paralelo, calcula traza en paralelo,
// decide cuál es válida, cancela la otra y registra tiempos.
func ejecucionEspeculativa(runID int, n, umbral int, outPath string, powData string, powDiff int, primesMax int) error {
	// preparar encabezado si no existe (solo en primera corrida)
	if runID == 1 {
		if _, err := os.Stat(outPath); os.IsNotExist(err) {
			if err := writeCSVHeader(outPath); err != nil {
				return err
			}
		}
	}

	// Contexto raíz y por rama
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	ctxA, cancelA := context.WithCancel(rootCtx)
	defer cancelA()
	ctxB, cancelB := context.WithCancel(rootCtx)
	defer cancelB()

	chA := make(chan resA, 1) // buffer 1 para no bloquear si termina antes de leer
	chB := make(chan resB, 1)

	// --- Medición de tiempo de pared (FIX): desde que lanzamos todo ---
	startGlobal := time.Now()

	// Lanzar rama A
	go func() {
		hash, nonce, d := SimularProofOfWork(ctxA, powData, powDiff)
		// FIX: sin default para no perder resultados
		chA <- resA{hash, nonce, d}
	}()

	// Lanzar rama B
	go func() {
		primes, d := EncontrarPrimos(ctxB, primesMax)
		chB <- resB{len(primes), d}
	}()

	// Lanzar traza concurrente
	traceCtx, traceCancel := context.WithCancel(rootCtx)
	traceDone := make(chan struct{})
	var trace int
	var tTrace time.Duration
	go func() {
		trace, tTrace = CalcularTrazaDeProductoDeMatrices(traceCtx, n)
		close(traceDone)
	}()

	// Esperar fin de la traza (decisión)
	<-traceDone

	// Decidir ganador
	var chosen string
	var tA, tB time.Duration
	var extra string

	if trace >= umbral {
		// Gana A: cancelar B
		chosen = "A"
		cancelB()

		select {
		case r := <-chA:
			tA = r.dur
			if r.nonce >= 0 {
				extra = fmt.Sprintf("nonce=%d", r.nonce)
			} else {
				extra = "A_cancelled"
			}
		case <-time.After(60 * time.Second):
			extra = "timeout_waiting_A"
		}
		// Si B alcanzó a terminar antes de la cancelación, podemos leerlo opcionalmente:
		select {
		case rB := <-chB:
			tB = rB.dur
		default:
		}
	} else {
		// Gana B: cancelar A
		chosen = "B"
		cancelA()

		select {
		case r := <-chB:
			tB = r.dur
			extra = fmt.Sprintf("primes=%d", r.primesCount)
		case <-time.After(60 * time.Second):
			extra = "timeout_waiting_B"
		}
		// Si A alcanzó a terminar antes de la cancelación, podemos leerlo opcionalmente:
		select {
		case rA := <-chA:
			tA = rA.dur
		default:
		}
	}

	// Tiempo total real de pared (FIX principal)
	total := time.Since(startGlobal)

	// Print resumen a stdout
	fmt.Printf("--- Run %d (spec) ---\n", runID)
	fmt.Printf("Trace=%d (t_trace=%s ms)\nChosen=%s\n", trace, ms(tTrace), chosen)
	fmt.Printf("tA=%s ms \ntB=%s ms \ntotal=%s ms \nextra=%s\n",
		ms(tA), ms(tB), ms(total), extra)

	// Guardar en CSV
	row := []string{
		strconv.Itoa(runID),
		"spec",
		strconv.Itoa(trace),
		chosen,
		ms(tTrace),
		ms(tA),
		ms(tB),
		ms(total),
		extra,
	}
	if err := appendCSV(outPath, row); err != nil {
		return err
	}

	// Limpieza
	traceCancel()
	rootCancel()
	time.Sleep(30 * time.Millisecond) // pequeña espera para dejar terminar goroutines

	return nil
}

// ejecucionSecuencial: calcula traza primero y luego ejecuta solo la rama ganadora.
func ejecucionSecuencial(runID int, n, umbral int, outPath string, powData string, powDiff int, primesMax int) error {
	if runID == 1 {
		if _, err := os.Stat(outPath); os.IsNotExist(err) {
			if err := writeCSVHeader(outPath); err != nil {
				return err
			}
		}
	}

	ctx := context.Background()
	startTotal := time.Now()

	trace, tTrace := CalcularTrazaDeProductoDeMatrices(ctx, n)
	var chosen string
	var tA, tB time.Duration
	var extra string

	if trace >= umbral {
		chosen = "A"
		_, nonce, d := SimularProofOfWork(ctx, powData, powDiff)
		tA = d
		if nonce >= 0 {
			extra = fmt.Sprintf("nonce=%d", nonce)
		} else {
			extra = "A_cancelled"
		}
	} else {
		chosen = "B"
		primes, d := EncontrarPrimos(ctx, primesMax)
		tB = d
		extra = fmt.Sprintf("primes=%d", len(primes))
	}

	total := time.Since(startTotal)

	fmt.Printf("--- Run %d (seq) ---\n", runID)
	fmt.Printf("Trace=%d (t_trace=%s ms)\nChosen=%s\n", trace, ms(tTrace), chosen)
	fmt.Printf("tA=%s ms \ntB=%s ms \ntotal=%s ms \nextra=%s\n",
		ms(tA), ms(tB), ms(total), extra)

	row := []string{
		strconv.Itoa(runID),
		"seq",
		strconv.Itoa(trace),
		chosen,
		ms(tTrace),
		ms(tA),
		ms(tB),
		ms(total),
		extra,
	}
	if err := appendCSV(outPath, row); err != nil {
		return err
	}
	return nil
}

// runBench: ejecuta `runs` veces spec y `runs` veces seq.
func runBench(runs int, n, umbral int, outPath string, powData string, powDiff int, primesMax int) error {
	// Borrar archivo previo si existe
	if _, err := os.Stat(outPath); err == nil {
		_ = os.Remove(outPath)
	}

	// Especulativos
	for i := 1; i <= runs; i++ {
		if err := ejecucionEspeculativa(i, n, umbral, outPath, powData, powDiff, primesMax); err != nil {
			return err
		}
	}
	// Secuenciales
	for i := 1; i <= runs; i++ {
		if err := ejecucionSecuencial(i, n, umbral, outPath, powData, powDiff, primesMax); err != nil {
			return err
		}
	}
	return nil
}

// ------------------------- Main -------------------------
func main() {
	seed := time.Now().UnixNano()
	rand.Seed(seed)

	// flags
	n := flag.Int("n", 100, "Dimensión de matrices (n)")
	umbral := flag.Int("umbral", 1000, "Umbral para decidir la rama")
	out := flag.String("out", "metrics.csv", "Archivo CSV de salida")
	mode := flag.String("mode", "spec", "Modo: spec | seq | bench")
	powData := flag.String("pow_in", "blockData", "Datos para proof-of-work")
	powDiff := flag.Int("pow_diff", 5, "Dificultad para proof-of-work (n ceros)")
	primesMax := flag.Int("primes_max", 50000, "Máximo para EncontrarPrimos")
	runs := flag.Int("runs", 30, "Número de repeticiones para bench")
	flag.Parse()

	switch *mode {
	case "spec":
		if _, err := os.Stat(*out); os.IsNotExist(err) {
			if err := writeCSVHeader(*out); err != nil {
				fmt.Printf("Error creando CSV: %v\n", err)
				return
			}
		}
		if err := ejecucionEspeculativa(1, *n, *umbral, *out, *powData, *powDiff, *primesMax); err != nil {
			fmt.Printf("Error spec: %v\n", err)
		}

	case "seq":
		if _, err := os.Stat(*out); os.IsNotExist(err) {
			if err := writeCSVHeader(*out); err != nil {
				fmt.Printf("Error creando CSV: %v\n", err)
				return
			}
		}
		if err := ejecucionSecuencial(1, *n, *umbral, *out, *powData, *powDiff, *primesMax); err != nil {
			fmt.Printf("Error seq: %v\n", err)
		}

	case "bench":
		fmt.Println("Seed:", seed)
		if err := runBench(*runs, *n, *umbral, *out, *powData, *powDiff, *primesMax); err != nil {
			fmt.Printf("Error bench: %v\n", err)
			return
		}

		// Calcular promedios y speedup desde el CSV
		f, err := os.Open(*out)
		if err != nil {
			fmt.Printf("No se pudo abrir CSV para calcular promedios: %v\n", err)
			return
		}
		defer f.Close()

		r := csv.NewReader(f)
		records, err := r.ReadAll()
		if err != nil {
			fmt.Printf("Error leyendo CSV: %v\n", err)
			return
		}

		var specTotals int64
		var seqTotals int64
		var specCount int64
		var seqCount int64
		for i := 1; i < len(records); i++ {
			rec := records[i]
			if len(rec) < 8 {
				continue
			}
			modeRec := rec[1]
			totalMs, _ := strconv.ParseInt(rec[7], 10, 64)
			if modeRec == "spec" {
				specTotals += totalMs
				specCount++
			} else if modeRec == "seq" {
				seqTotals += totalMs
				seqCount++
			}
		}
		// FIX: condicional correcto
		if specCount == 0 || seqCount == 0 {
			fmt.Println("No hay suficientes datos para calcular Speedup")
			return
		}
		avgSpec := float64(specTotals) / float64(specCount)
		avgSeq := float64(seqTotals) / float64(seqCount)
		speedup := avgSeq / avgSpec
		fmt.Printf("---- BENCH RESULTADOS ----\n")
		fmt.Printf("Promedio Especulativo (ms): %.2f\n", avgSpec)
		fmt.Printf("Promedio Secuencial (ms): %.2f\n", avgSeq)
		fmt.Printf("Speedup: %.4f\n", speedup)

	default:
		fmt.Println("Modo desconocido. Usa spec | seq | bench")
	}
}
