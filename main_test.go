package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// TestGetBookmarksFilePath проверяет функцию получения пути к файлу закладок
func TestGetBookmarksFilePath(t *testing.T) {
	path, err := getBookmarksFilePath()

	// Проверяем только логику функции, не требуя наличия реального файла
	if err != nil {
		// Если файл не найден, это нормально для тестового окружения
		// Проверяем, что путь содержит ожидаемые компоненты
		userProfile := os.Getenv("USERPROFILE")
		expectedPath := filepath.Join(userProfile, "AppData", "Local", "Google", "Chrome", "User Data", "Default", "Bookmarks")

		if expectedPath != path {
			t.Errorf("Неверный путь к файлу закладок. Получено: %s, ожидалось: %s", path, expectedPath)
		}
	} else {
		// Если файл найден, проверяем, что путь не пустой
		if path == "" {
			t.Error("Получен пустой путь к файлу закладок")
		}
	}
}

// TestExtractBookmarks проверяет функцию извлечения закладок
func TestExtractBookmarks(t *testing.T) {
	// Создаем тестовую структуру закладок
	bookmarks := []BookmarkItem{
		{
			ID:        "1",
			Name:      "Тестовая папка",
			Type:      "folder",
			DateAdded: "13311432144000000",
			Children: []BookmarkItem{
				{
					ID:        "2",
					Name:      "Тестовая закладка 1",
					Type:      "url",
					URL:       "https://example.com",
					DateAdded: "13311432144000001",
				},
				{
					ID:        "3",
					Name:      "Тестовая закладка 2",
					Type:      "url",
					URL:       "https://example.org",
					DateAdded: "13311432144000002",
				},
			},
		},
		{
			ID:        "4",
			Name:      "Тестовая закладка 3",
			Type:      "url",
			URL:       "https://example.net",
			DateAdded: "13311432144000003",
		},
	}

	// Извлекаем закладки
	extracted := extractBookmarks(bookmarks)

	// Проверяем количество извлеченных закладок
	if len(extracted) != 3 {
		t.Errorf("Неверное количество извлеченных закладок. Получено: %d, ожидалось: %d", len(extracted), 3)
	}

	// Проверяем, что все закладки имеют тип "url"
	for i, bookmark := range extracted {
		if bookmark.Type != "url" {
			t.Errorf("Закладка %d имеет неверный тип. Получено: %s, ожидалось: %s", i, bookmark.Type, "url")
		}
	}
}

// TestInitDB проверяет функцию инициализации базы данных
func TestInitDB(t *testing.T) {
	// Создаем временный файл для тестовой базы данных
	tempDir := os.TempDir()
	dbPath := filepath.Join(tempDir, "test_bookmarks.db")

	// Удаляем файл, если он существует
	os.Remove(dbPath)

	// Инициализируем базу данных
	db, err := initDB(dbPath)
	if err != nil {
		t.Fatalf("Ошибка инициализации базы данных: %v", err)
	}
	defer db.Close()

	// Проверяем, что таблица bookmarks создана
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='bookmarks'").Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			t.Error("Таблица bookmarks не создана")
		} else {
			t.Errorf("Ошибка проверки наличия таблицы: %v", err)
		}
	}

	// Проверяем структуру таблицы
	rows, err := db.Query("PRAGMA table_info(bookmarks)")
	if err != nil {
		t.Fatalf("Ошибка получения информации о таблице: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notnull, pk int
		var name, type_, dflt_value string
		err = rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk)
		if err != nil {
			t.Fatalf("Ошибка сканирования строки: %v", err)
		}
		columns[name] = true
	}

	// Проверяем наличие всех необходимых столбцов
	requiredColumns := []string{"id", "name", "url", "date_added"}
	for _, col := range requiredColumns {
		if !columns[col] {
			t.Errorf("В таблице отсутствует столбец %s", col)
		}
	}

	// Удаляем временный файл
	os.Remove(dbPath)
}
