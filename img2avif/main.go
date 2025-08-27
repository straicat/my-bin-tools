package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const helpText = `img2avif - 图片转AVIF格式工具

使用方法:
  img2avif [选项] <图片文件路径1> [图片文件路径2] ...

选项:
  -w int    设置图片最大宽度 (不指定则不限制宽度)
  -h        显示此帮助信息

示例:
  img2avif image.jpg                    # 不限制宽度，保持原始尺寸
  img2avif -w 940 image.jpg            # 限制最大宽度为940
  img2avif -w 1920 image.jpg           # 限制最大宽度为1920
  img2avif -w 800 image1.jpg image2.png # 批量转换，最大宽度800

支持的输入格式: jpg, jpeg, png, gif, bmp, webp
输出格式: AVIF (使用MD5哈希命名)
`

func main() {
	var maxWidth int
	var showHelp bool

	flag.IntVar(&maxWidth, "w", 0, "设置图片最大宽度 (不指定则不限制)")
	flag.BoolVar(&showHelp, "h", false, "显示此帮助信息")
	flag.Parse()

	if showHelp {
		fmt.Print(helpText)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "错误: 请提供至少一个图片文件路径\n\n")
		fmt.Fprintf(os.Stderr, "%s", helpText)
		os.Exit(1)
	}

	for _, inputPath := range args {
		outputPath, err := processImage(inputPath, maxWidth)
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

func processImage(inputPath string, maxWidth int) (string, error) {
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
		if err := convertToAvif(inputPath, tempPath, ext, maxWidth); err != nil {
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

func convertToAvif(inputPath, outputPath, ext string, maxWidth int) error {
	var cmd *exec.Cmd

	var scaleFilter string
	if maxWidth > 0 {
		scaleFilter = fmt.Sprintf("scale=w=min(iw\\,%d):h=-2", maxWidth)
	} else {
		scaleFilter = "scale=iw:ih"
	}

	switch ext {
	case ".gif":
		cmd = exec.Command("ffmpeg", "-i", inputPath, "-vf", scaleFilter, "-vsync", "vfr", "-pix_fmt", "rgb8", "-loop", "0", "-c:v", "libaom-av1", "-crf", "32", outputPath)
	case ".png":
		cmd = exec.Command("ffmpeg", "-i", inputPath, "-f", "lavfi", "-i", "color=white:s=1x1", "-filter_complex", fmt.Sprintf("[1][0]scale2ref[bg][img];[bg][img]overlay,%s", scaleFilter), "-c:v", "libaom-av1", "-pix_fmt", "yuv420p", "-crf", "32", outputPath)
	default:
		cmd = exec.Command("ffmpeg", "-i", inputPath, "-vf", scaleFilter, "-c:v", "libaom-av1", "-crf", "32", outputPath)
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
