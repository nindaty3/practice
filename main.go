package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

func main() {
	// Проверяем, передал ли пользователь имя файла
	if len(os.Args) < 2 {
		fmt.Println("Использование: go run main.go hosts.txt")
		return
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal("Ошибка открытия:", err)
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		host := scanner.Text()
		if host != "" {
			hosts = append(hosts, host)
		}
	}

	fmt.Printf("Загружено хостов: %d\n", len(hosts))
	for _, h := range hosts {
		fmt.Println(" -", h)
	}
}
