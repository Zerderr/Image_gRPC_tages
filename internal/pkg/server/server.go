package server

import (
	"bytes"
	"context"
	"github.com/djherbis/times"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"grpc_image_test/internal/pkg/pb"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

//Создаем канал, при переполнении и попытке записи в него горутина блокируется, пока канал не освободится,
//В начале метода записываем в канал, в конце убираем из канал элемент

var listLimit = make(chan int, 100)
var uploadLimit = make(chan int, 10)
var downloadLimit = make(chan int, 10)

type Implementation struct {
	pb.UnimplementedImageServiceServer
	imageStore ImageStorage
}

func NewImplementation(imageStore ImageStorage) *Implementation {
	return &Implementation{imageStore: imageStore}
}

func (i *Implementation) UploadImage(stream pb.ImageService_UploadImageServer) error {
	//Если канал переполнен, горутина блокируется, пока другая горутина не завершится, освобождая канал
	uploadLimit <- 1
	//Получаем информацию о фото из стрима
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Unknown, "Cannot receive image info: %v", err)
	}
	imageName := req.GetInfo().GetName()
	imageType := req.GetInfo().GetType()
	imageData := bytes.Buffer{}
	var imageSize uint32 = 0
	//Передаем чанками по стриму
	for {
		request, err := stream.Recv()
		if err == io.EOF {
			log.Print("no more data")
			break
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "Cannot receive image chunk: %v", err)
		}
		chunk := request.GetChunkData()
		size := uint32(len(chunk))
		imageSize += size
		_, err = imageData.Write(chunk)
		if err != nil {
			return status.Errorf(codes.Unknown, "Cannot write data chunk: %v", err)
		}
	}
	//Сохраняем файл на сервере
	imageId, err := i.imageStore.Save(imageName, imageData, imageType)
	if err != nil {
		return status.Errorf(codes.Unknown, "Cannot save image: %v", err)
	}
	//Отправляем ответ клиенту
	res := &pb.UploadImageResponse{
		Id:   imageId,
		Size: imageSize,
	}
	err = stream.SendAndClose(res)
	if err != nil {
		return status.Errorf(codes.Unknown, "Cannot send response: %v", err)
	}
	//Убираем элемент из канала
	<-uploadLimit
	log.Println("Image uploaded")
	return nil
}

func (i *Implementation) ListImage(_ context.Context, _ *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
	//Заносим элемент в канал
	listLimit <- 1
	//Читаем файлы в дирректории
	files, err := ioutil.ReadDir(ImageFolder)
	if err != nil {
		log.Println(err)
	}
	//Массив имен и времени создания/обновления файлов
	imageInfo := make([]*pb.ImageName, 0)
	for _, file := range files {
		t, err := times.Stat(ImageFolder + "/" + file.Name())
		if err != nil {
			log.Printf("Couldn't access file: %v\n", err)
		}
		//Ответ в timestamp
		image := &pb.ImageName{
			Name:       file.Name(),
			CreateTime: timestamppb.New(t.BirthTime()),
			UpdateTime: timestamppb.New(t.ChangeTime()),
		}
		imageInfo = append(imageInfo, image)

	}
	//Убираем элемент из канала
	<-listLimit
	log.Println("Files have been read")
	return &pb.ListImagesResponse{Images: imageInfo}, nil
}

func (i *Implementation) DownloadImage(req *pb.DownloadImageRequest, server pb.ImageService_DownloadImageServer) error {
	var mu sync.Mutex
	//Так как получаем доступ к файлам, блокируем доступ к ним для других горутин
	mu.Lock()
	defer mu.Unlock()

	downloadLimit <- 1
	name := req.GetName()

	//Считываем данные файла с сервера
	data, err := os.ReadFile(ImageFolder + "/" + name)
	if err != nil {
		return status.Errorf(codes.Unknown, "Cannot open file: %v", err)
	}

	info := &pb.ImageInfo{
		Name: name,
	}
	//Отсылаем данные
	res := &pb.DownloadImageResponse{
		Info:      info,
		ChunkData: data,
	}
	imageData := bytes.NewBuffer(data)

	err = server.Send(res)
	if err != nil {
		return status.Errorf(codes.Unknown, "Cannot send image: %v", err)
	}
	err = i.imageStore.SaveDownloaded(name, *imageData)
	if err != nil {
		return status.Errorf(codes.Unknown, "Cannot save image: %v", err)
	}
	<-downloadLimit
	log.Println("Image downloaded")
	return nil

}
