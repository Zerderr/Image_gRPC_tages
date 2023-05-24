package server

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"os"
	"sync"
	"time"
)

type ImageStorage interface {
	Save(name string, imageData bytes.Buffer, imageType string) (string, error)
	SaveDownloaded(name string, imageData bytes.Buffer) error
}

type diskImageStorage struct {
	mutex       sync.Mutex
	imageFolder string
	//Мапа для хранения картинок, ключ id, который генерим в Save, значение информация о картинке
	Images map[string]*ImageInfo
}

type ImageInfo struct {
	Name       string
	UploadTime time.Time
	Type       string
	Path       string
}

func NewDiskImageStorage(imageFolder string) *diskImageStorage {
	return &diskImageStorage{
		imageFolder: imageFolder,
		Images:      make(map[string]*ImageInfo),
	}
}

func (d *diskImageStorage) Save(name string, imageData bytes.Buffer, imageType string) (string, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	//Генерим id
	imageId, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("cannot generate id: %w", err)
	}
	//Создаем файл

	imagePath := fmt.Sprintf("%s/%s.%s", d.imageFolder, name, imageType)

	file, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("cannot create file: %w", err)
	}
	//Записываем в файл
	_, err = imageData.WriteTo(file)
	if err != nil {
		return "", fmt.Errorf("cannot write data to file: %w", err)
	}

	//Заполняем мапу данными о файлах, в целом мы их и так можем посмотреть через директорию, но если файл удален, то у нас сохранится информация о нем в мапе
	d.Images[imageId.String()] = &ImageInfo{
		Name:       name,
		UploadTime: time.Now(),
		Type:       imageType,
		Path:       imagePath,
	}
	return imageId.String(), nil
}

func (d *diskImageStorage) SaveDownloaded(name string, imageData bytes.Buffer) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	//Создаем файл
	imagePath := fmt.Sprintf("%s/%s", downloadedFolder, name)
	file, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("cannot create file: %w", err)
	}
	//Записываем в файл
	_, err = imageData.WriteTo(file)
	if err != nil {
		return fmt.Errorf("cannot write data to file: %w", err)
	}

	return nil
}
