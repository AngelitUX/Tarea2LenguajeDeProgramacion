# Control 2 — Lenguajes de Programación II  
**Tema:** Ejecución Especulativa en Go  
**Profesor:** Alonso Inostrosa Psijas  
**Fecha de entrega:** 28/10/2025  

---

##  Integrantes
- Thean Orlandi
- Lucas Orellana
- Angel Pino

---

##  Descripción del proyecto
Este proyecto implementa **ejecución especulativa** en el lenguaje **Go**, utilizando **goroutines** y **canales** (`channels`) para manejar la concurrencia y la sincronización.  
El objetivo fue comparar el rendimiento de una ejecución tradicional **secuencial** con una **especulativa**, donde ambas ramas de cómputo (A y B) se ejecutan en paralelo mientras se evalúa una condición costosa.

### Concepto de ejecución especulativa
La idea es lanzar tareas en paralelo antes de saber cuál de ellas será realmente necesaria.  
Cuando se determina el resultado de la condición, la rama correcta se mantiene y la otra se **cancela** de manera controlada utilizando **`context.Context`**.

---

##  Tecnologías y herramientas
- Lenguaje: **Go 1.20+**
- Concurrencia: **Goroutines** y **Channels**
- Sincronización y cancelación: **context.WithCancel**
- Almacenamiento de métricas: **CSV**
- Análisis: **Promedios y Speedup**

---

##  Estructura del código
```
control2/
├─ main.go               # Código principal (con todas las funciones y modos)
├─ go.mod                # Módulo Go
├─ control2.exe          # Ejecutable generado 
├─ bench_metrics.csv     # Resultados del benchmark
```

---

##  Instrucciones de uso

### Compilación
```bash
go mod init control2
go build -o control2.exe main.go
```

### Ejecución 
#### Modo especulativo
```bash
.\control2.exe -mode spec -n 120 -umbral 800 -out spec_metrics.csv -pow_diff 5 -primes_max 50000
```

#### Modo secuencial
```bash
.\control2.exe -mode seq -n 120 -umbral 800 -out seq_metrics.csv -pow_diff 5 -primes_max 50000
```

#### Benchmark (30 repeticiones automáticas)
```bash
.\control2.exe -mode bench -runs 30 -n 120 -umbral 800 -out bench_metrics.csv -pow_diff 5 -primes_max 50000
```

---

##  Parámetros del programa
| Parámetro | Descripción | Ejemplo |
|------------|-------------|----------|
| `-n` | Dimensión de matrices NxN para la traza (condición) | `120` |
| `-umbral` | Umbral para decidir la rama ganadora | `800` |
| `-pow_diff` | Dificultad del Proof of Work | `5` |
| `-primes_max` | Límite superior para búsqueda de primos | `50000` |
| `-mode` | Modo de ejecución (`spec`, `seq`, `bench`) | `bench` |
| `-runs` | Número de repeticiones en modo benchmark | `30` |
| `-out` | Archivo CSV de salida con métricas | `bench_metrics.csv` |

---

##  Análisis de rendimiento

Después de ejecutar el benchmark 30 veces por cada modo, se obtuvieron los siguientes resultados promedio:

| Modo | Tiempo promedio (ms) |
|------|----------------------|
| **Secuencial** | 691.93 |
| **Especulativo** | 697.27 |
| **Speedup (T_seq / T_spec)** | **0.99×** |

###  Interpretación
El resultado muestra un **Speedup ≈ 1**, lo que significa que el rendimiento de ambas estrategias fue prácticamente igual.  
Esto ocurre porque la función de decisión (`CalcularTrazaDeProductoDeMatrices`) no tarda lo suficiente para que las ramas especulativas aprovechen tiempo extra antes de conocerse la condición.  
En otras palabras, el tiempo adicional de coordinación (creación de goroutines y canales) iguala la posible ganancia.

---

##  Conclusiones

1. **El patrón especulativo funciona correctamente:**  
   - Las ramas A y B se ejecutan concurrentemente.  
   - Cuando se conoce la condición, se **cancela la rama perdedora** mediante `context.WithCancel()`.  
   - Los resultados se comunican con canales (`chan`).  

2. **El rendimiento depende de la relación entre el costo de la condición y las ramas.**  
   - Si la condición es costosa, la ejecución especulativa puede ofrecer **mejoras reales de rendimiento**.  
   - Si las ramas son más pesadas o similares en tiempo, el overhead de concurrencia neutraliza el beneficio.  

3. **La cancelación controlada evita desperdicio de recursos**, garantizando que las goroutines terminen de forma ordenada.

4. **El uso de goroutines y canales simplifica la paralelización**, demostrando la potencia del modelo de concurrencia de Go.

<img width="1200" height="750" alt="01_medias_beneficio" src="https://github.com/user-attachments/assets/568359b5-f3d2-4e4c-bbbc-601135e47e79" />

---
