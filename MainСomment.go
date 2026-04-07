package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"
)

// Структура результата пинга
type PingResult struct {
	Timestamp time.Time
	Host      string
	Success   bool
	Latency   time.Duration
}

func main() {
	// Парсинг аргументов командной строки
	hostsFile := flag.String("f", "", "Hosts file")
	count := flag.Int("c", 3, "Checks")
	monitor := flag.Bool("monitor", false, "Monitor")
	flag.Parse()

	// Проверка обязательного аргумента
	if *hostsFile == "" {
		fmt.Println("Need -f")
		os.Exit(1)
	}

	// Чтение списка хостов из файла
	hosts, err := readHosts(*hostsFile)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Выбор режима работы
	if *monitor {
		runMonitor(hosts)
	} else {
		runOneTime(hosts, *count)
	}
}

// Читает файл с хостами игнорируя пустые строки и комментарии
func readHosts(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)

	// Построчное чтение файла
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Пропуск пустых строк и комментариев
		if line != "" && !strings.HasPrefix(line, "#") {
			hosts = append(hosts, line)
		}
	}
	return hosts, scanner.Err()
}

// Одноразовая проверка хостов
func runOneTime(hosts []string, count int) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Host\tStatus\tLatency\tAttempt")

	// Перебор хостов и попыток
	for _, host := range hosts {
		for i := 1; i <= count; i++ {
			success, latency := pingHost(host)
			status, latencyText := formatResult(success, latency)

			fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\n", host, status, latencyText, i, count)

			// Задержка между попытками
			if i < count {
				time.Sleep(1 * time.Second)
			}
		}
	}
	w.Flush()
}

// Режим постоянного мониторинга
func runMonitor(hosts []string) {
	// Открытие файла логов
	log, err := os.OpenFile("monitor.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Log error:", err)
		os.Exit(1)
	}
	defer log.Close()

	// Контекст для управления остановкой горутин
	ctx, cancel := context.WithCancel(context.Background())

	// Канал для передачи результатов
	results := make(chan PingResult, 10)

	// WaitGroup для ожидания завершения воркеров
	var wg sync.WaitGroup

	// Consumer: читает результаты и пишет в лог
	go func() {
		for r := range results {
			printResult(r)
			writeLog(log, r)
		}
	}()

	// Запуск горутин для каждого хоста
	for _, host := range hosts {
		wg.Add(1)
		go monitorHost(ctx, &wg, host, results)
	}

	// Канал для ловли Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)

	// Ожидание сигнала завершения
	<-sig

	// Завершение работы
	cancel()       // Остановка всех горутин
	wg.Wait()      // Ожидание завершения
	close(results) // Закрытие канала
}

// Горутина мониторинга одного хоста
func monitorHost(ctx context.Context, wg *sync.WaitGroup, host string, results chan<- PingResult) {
	defer wg.Done()

	// Функция отправки результата
	send := func() {
		success, latency := pingHost(host)
		results <- PingResult{time.Now(), host, success, latency}
	}

	// Первый пинг сразу
	send()

	// Таймер для периодических проверок
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Основной цикл либо остановка, либо новый пинг
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

// Проверка доступности хоста через TCP 80 порт
func pingHost(host string) (bool, time.Duration) {
	start := time.Now()

	// Попытка TCP соединения с таймаутом
	conn, err := net.DialTimeout("tcp", host+":80", 2*time.Second)
	if err != nil {
		return false, 0
	}
	conn.Close()

	return true, time.Since(start)
}

// Форматирует результат в строку
func formatResult(success bool, latency time.Duration) (string, string) {
	if !success {
		return "Timeout", "0ms"
	}
	return "OK", fmt.Sprintf("%dms", latency.Milliseconds())
}

// Вывод результата в консоль
func printResult(r PingResult) {
	status, latencyText := formatResult(r.Success, r.Latency)
	fmt.Printf("%s | %s | %s | %s\n",
		r.Timestamp.Format("2006-01-02 15:04:05"),
		r.Host,
		status,
		latencyText)
}

// Запись результата в лог-файл
func writeLog(file *os.File, r PingResult) {
	status, latencyText := formatResult(r.Success, r.Latency)

	line := fmt.Sprintf("%s | %s | %s | %s\n",
		r.Timestamp.Format("2006-01-02 15:04:05"),
		r.Host,
		status,
		latencyText)

	file.WriteString(line)
}
