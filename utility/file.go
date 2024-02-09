package utility

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/c-m3-codin/gsched/models"
)

func Filemd5sum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (c *Config) LoadScheduleFile() (scheduleFile models.ScheduleFile) {

	fmt.Println("\n\n\n Loading new file ...schedule changed\n\n\n ")
	file, err := os.Open(c.ScheduleFile)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// Decode JSON from the file

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&scheduleFile)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}
	return scheduleFile

}
