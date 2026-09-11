package server

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	errImportFileTooLarge = errors.New("导入文件超过 20 MiB")
	errImportReadFailed   = errors.New("读取导入文件失败")
)

// isRequestBodyTooLarge 判断错误是否由 MaxBytesReader 的 21 MiB 请求体上限触发。
func isRequestBodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// readImportFile 有界读取导入文件字段：恰好 20 MiB 允许，20 MiB+1 立即返回超限；
// 正常 EOF 结束，真实读取错误返回带原因的错误，不把非 EOF 当作正常结束。
func readImportFile(file multipart.File) ([]byte, error) {
	data := make([]byte, 0, 64*1024)
	buf := make([]byte, 64*1024)
	for {
		n, rerr := file.Read(buf)
		if n > 0 {
			if int64(len(data))+int64(n) > MaxImportFileBytes {
				return nil, errImportFileTooLarge
			}
			data = append(data, buf[:n]...)
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return data, nil
			}
			return nil, fmt.Errorf("%w: %v", errImportReadFailed, rerr)
		}
	}
}
