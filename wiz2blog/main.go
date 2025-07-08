package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	fmt.Println("开始处理...")
	if err := process(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("处理完成!")
}

func process() error {
	fmt.Println("正在查找 Markdown 文件...")
	files, err := filepath.Glob("*.md")
	if err != nil {
		return fmt.Errorf("查找 Markdown 文件失败: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("当前目录下没有找到 Markdown 文件")
	}

	if len(files) > 1 {
		return fmt.Errorf("当前目录下存在多个 Markdown 文件，请只保留一个")
	}

	mdFile := files[0]
	fmt.Printf("找到文件: %s\n", mdFile)
	return processFile(mdFile)
}

func processFile(filename string) error {
	fmt.Printf("正在读取文件内容...\n")
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取文件失败 %s: %v", filename, err)
	}

	lines := strings.Split(string(content), "\n")

	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		fmt.Println("检测到并移除了第一行的标题")
		lines = lines[1:]
	}

	fmt.Println("正在生成文件头...")
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	now := time.Now().Format("2006-01-02T15:04:05")
	header := fmt.Sprintf("---\ntitle: %s\ndate: %s\ntags: [ ]\n---\n\n", title, now)
	newContent := header + strings.Join(lines, "\n")

	fmt.Println("开始处理图片...")
	newContent, err = processImages(newContent)
	if err != nil {
		return fmt.Errorf("处理图片失败: %v", err)
	}

	fmt.Printf("正在保存文件: %s\n", filename)
	if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("保存文件失败 %s: %v", filename, err)
	}

	return nil
}

func getMD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func processImages(content string) (string, error) {
	if _, err := os.Stat("images"); os.IsNotExist(err) {
		fmt.Println("未找到 images 目录，跳过图片处理")
		return content, nil
	}

	fmt.Println("正在扫描图片文件...")
	imageFiles, err := filepath.Glob("images/*")
	if err != nil {
		return "", fmt.Errorf("扫描图片文件失败: %v", err)
	}

	if len(imageFiles) == 0 {
		fmt.Println("images 目录中没有图片文件")
		return content, nil
	}

	fmt.Printf("找到 %d 个图片文件\n", len(imageFiles))
	re := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	fileMapping := make(map[string]string)

	for _, imgFile := range imageFiles {
		ext := strings.ToLower(filepath.Ext(imgFile))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".avif" {
			fmt.Printf("跳过不支持的文件: %s\n", imgFile)
			continue
		}

		fmt.Printf("正在处理图片: %s\n", imgFile)

		var newName string
		var outputPath string

		if ext == ".avif" {
			data, err := os.ReadFile(imgFile)
			if err != nil {
				return "", fmt.Errorf("读取图片失败 %s: %v", imgFile, err)
			}
			newName = getMD5Hash(data) + ".avif"
			outputPath = filepath.Join("images", newName)

			fmt.Printf("文件已是 AVIF 格式，重命名: %s -> %s\n", imgFile, outputPath)
			if imgFile != outputPath {
				if err := os.Rename(imgFile, outputPath); err != nil {
					return "", fmt.Errorf("重命名文件失败 %s: %v", imgFile, err)
				}
			}
		} else {
			tempOutputPath := filepath.Join("images", "temp_"+filepath.Base(imgFile)+".avif")

			fmt.Printf("转换为 AVIF 格式: %s -> %s\n", imgFile, tempOutputPath)
			var cmd *exec.Cmd
			if ext == ".gif" {
				cmd = exec.Command("ffmpeg", "-i", imgFile, "-vsync", "vfr", "-pix_fmt", "rgb8", "-loop", "0", "-c:v", "libaom-av1", "-crf", "32", tempOutputPath)
			} else if ext == ".png" {
				cmd = exec.Command("ffmpeg", "-i", imgFile, "-f", "lavfi", "-i", "color=white:s=1x1", "-filter_complex", "[1][0]scale2ref[bg][img];[bg][img]overlay", "-c:v", "libaom-av1", "-pix_fmt", "yuv420p", "-crf", "32", tempOutputPath)
			} else {
				cmd = exec.Command("ffmpeg", "-i", imgFile, "-c:v", "libaom-av1", "-crf", "32", tempOutputPath)
			}

			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("转换图片失败 %s: %v", imgFile, err)
			}

			avifData, err := os.ReadFile(tempOutputPath)
			if err != nil {
				return "", fmt.Errorf("读取转换后的AVIF文件失败 %s: %v", tempOutputPath, err)
			}

			newName = getMD5Hash(avifData) + ".avif"
			outputPath = filepath.Join("images", newName)

			if tempOutputPath != outputPath {
				if err := os.Rename(tempOutputPath, outputPath); err != nil {
					return "", fmt.Errorf("重命名转换后的文件失败 %s: %v", tempOutputPath, err)
				}
			}

			fmt.Printf("删除原始图片: %s\n", imgFile)
			if err := os.Remove(imgFile); err != nil {
				return "", fmt.Errorf("删除原始图片失败 %s: %v", imgFile, err)
			}
		}

		fileMapping[filepath.Base(imgFile)] = newName
	}

	fmt.Println("正在更新 Markdown 中的图片链接...")
	newContent := re.ReplaceAllStringFunc(content, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}

		altText := parts[1]
		origPath := parts[2]
		origFilename := filepath.Base(origPath)

		if newName, ok := fileMapping[origFilename]; ok {
			newPath := filepath.Join("images", newName)
			fmt.Printf("更新图片链接: %s -> %s\n", origPath, newPath)
			return fmt.Sprintf("![%s](%s)", altText, newPath)
		}

		return match
	})

	return newContent, nil
}
