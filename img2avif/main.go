package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("使用方法: img2avif <图片文件路径1> [图片文件路径2] ...")
		os.Exit(1)
	}

	for _, inputPath := range os.Args[1:] {
		outputPath, err := processImage(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "处理图片失败 %s: %v\n", inputPath, err)
			continue
		}
		fmt.Printf("%s -> %s\n", filepath.Base(inputPath), filepath.Base(outputPath))
	}
}

func getMD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func processImage(inputPath string) (string, error) {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("文件不存在: %s", inputPath)
	}

	ext := strings.ToLower(filepath.Ext(inputPath))

	if ext == ".avif" {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return "", fmt.Errorf("读取文件失败: %v", err)
		}
		md5Name := getMD5Hash(data)
		outputName := md5Name + ".avif"
		outputPath := filepath.Join(filepath.Dir(inputPath), outputName)

		if err := copyFile(inputPath, outputPath); err != nil {
			return "", fmt.Errorf("复制文件失败: %v", err)
		}
		return outputPath, nil
	} else {
		baseName := filepath.Base(inputPath)
		tempFileName := "temp_" + baseName + ".avif"
		tempPath := filepath.Join(filepath.Dir(inputPath), tempFileName)
		if err := convertToAvif(inputPath, tempPath, ext); err != nil {
			return "", fmt.Errorf("转换失败: %v", err)
		}

		data, err := os.ReadFile(tempPath)
		if err != nil {
			return "", fmt.Errorf("读取转换后文件失败: %v", err)
		}

		md5Name := getMD5Hash(data)
		outputName := md5Name + ".avif"
		outputPath := filepath.Join(filepath.Dir(inputPath), outputName)

		if err := os.Rename(tempPath, outputPath); err != nil {
			return "", fmt.Errorf("重命名文件失败: %v", err)
		}

		return outputPath, nil
	}
}

func convertToAvif(inputPath, outputPath, ext string) error {
	var cmd *exec.Cmd

	switch ext {
	case ".gif":
		cmd = exec.Command("ffmpeg", "-i", inputPath, "-vsync", "vfr", "-pix_fmt", "rgb8", "-loop", "0", "-c:v", "libaom-av1", "-crf", "32", outputPath)
	case ".png":
		cmd = exec.Command("ffmpeg", "-i", inputPath, "-f", "lavfi", "-i", "color=white:s=1x1", "-filter_complex", "[1][0]scale2ref[bg][img];[bg][img]overlay", "-c:v", "libaom-av1", "-pix_fmt", "yuv420p", "-crf", "32", outputPath)
	default:
		cmd = exec.Command("ffmpeg", "-i", inputPath, "-c:v", "libaom-av1", "-crf", "32", outputPath)
	}

	return cmd.Run()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
