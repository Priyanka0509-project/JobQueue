package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type CloudinaryClient struct {
	cld *cloudinary.Cloudinary
}

func NewCloudinaryClient() (*CloudinaryClient, error) {
	cloudName := strings.TrimSpace(os.Getenv("CLOUDINARY_CLOUD_NAME"))
	apiKey := strings.TrimSpace(os.Getenv("CLOUDINARY_API_KEY"))
	apiSecret := strings.TrimSpace(os.Getenv("CLOUDINARY_API_SECRET"))

	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("missing Cloudinary env vars: CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, CLOUDINARY_API_SECRET")
	}

	cld, err := cloudinary.NewFromParams(
		cloudName,
		apiKey,
		apiSecret,
	)
	if err != nil {
		return nil, err
	}

	return &CloudinaryClient{cld: cld}, nil
}

func (c *CloudinaryClient) Upload(data []byte, fileName string) (string, error) {
	resp, err := c.cld.Upload.Upload(context.TODO(), bytes.NewReader(data), uploader.UploadParams{
		PublicID:     fileName,
		ResourceType: "auto",
	})

	if err != nil {
		fmt.Println("UPLOAD ERROR:", err)
		return "", err
	}
	fmt.Println("FULL RESPONSE:", resp)
	fmt.Println("Cloudinary URL:", resp.SecureURL)
	return resp.SecureURL, nil
}

func (c *CloudinaryClient) Download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cloudinary download failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}
