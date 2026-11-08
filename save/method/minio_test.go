package method

import (
	"os"
	"testing"

	"github.com/Wenaixi/submatrix/config"
)

func TestUploadToS3(t *testing.T) {
	// 需要本地 MinIO 服务（127.0.0.1:9000），设置 MINIO_TEST=1 才运行
	if os.Getenv("MINIO_TEST") == "" {
		t.Skip("跳过 MinIO 测试（设置 MINIO_TEST=1 且本机 9000 端口运行 MinIO 时启用）")
	}
	config.GlobalConfig.S3Endpoint = "127.0.0.1:9000"
	config.GlobalConfig.S3AccessID = "123"
	config.GlobalConfig.S3SecretKey = "123"
	config.GlobalConfig.S3Bucket = "public"
	config.GlobalConfig.S3UseSSL = false
	type args struct {
		data     []byte
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TEST MINIO",
			args: args{
				data:     []byte("test"),
				filename: "test.yaml",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UploadToS3(tt.args.data, tt.args.filename); (err != nil) != tt.wantErr {
				t.Errorf("UploadToS3() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
