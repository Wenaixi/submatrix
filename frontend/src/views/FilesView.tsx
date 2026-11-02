import React, { useState, useEffect } from 'react';
import { FileItem, getFilesData } from '../api/client';
import { FileText, Download, Copy, Eye, X, RefreshCw } from 'lucide-react';
import { useToast } from '../components/Toast';

export const FilesView: React.FC = () => {
  const { showToast } = useToast();
  const [files, setFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [previewFile, setPreviewFile] = useState<{ name: string; content: string } | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const loadFiles = async () => {
    setLoading(true);
    try {
      const res = await getFilesData();
      setFiles(res.files || []);
    } catch (e: any) {
      showToast(`加载文件列表失败: ${e.message}`, true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadFiles();
  }, []);

  const handleCopy = (rawUrl: string) => {
    const full = location.origin + rawUrl;
    if (navigator.clipboard) {
      navigator.clipboard.writeText(full).then(() => showToast("文件直链已复制"));
    }
  };

  const handlePreview = async (file: FileItem) => {
    setPreviewLoading(true);
    setPreviewFile({ name: file.name, content: "正在读取文件内容..." });
    try {
      const res = await fetch(file.raw_url);
      const text = await res.text();
      setPreviewFile({ name: file.name, content: text });
    } catch (e: any) {
      setPreviewFile({ name: file.name, content: `读取失败: ${e.message}` });
    } finally {
      setPreviewLoading(false);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      <div style={{
        padding: '16px 20px',
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        flexWrap: 'wrap',
        gap: '12px'
      }}>
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 500, color: 'var(--text-primary)' }}>
            内置文件服务 (Files Manager)
          </h2>
          <p style={{ fontSize: '13px', color: 'var(--text-secondary)', marginTop: '2px' }}>
            管理 output 目录下的订阅生成文件、统计报告与规则集，单页无缝内置
          </p>
        </div>

        <button onClick={loadFiles} className="btn-ghost" disabled={loading}>
          <RefreshCw size={13} className={loading ? "animate-spin" : ""} /> 刷新列表
        </button>
      </div>

      {/* 文件表格列表 */}
      <div style={{
        background: 'var(--surface-2)',
        border: '1px solid var(--border)',
        overflowX: 'auto'
      }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '13px' }}>
          <thead>
            <tr style={{ background: 'var(--surface-1)', borderBottom: '1px solid var(--border)' }}>
              <th style={{ padding: '10px 16px', fontWeight: 500 }}>文件名</th>
              <th style={{ padding: '10px 16px', fontWeight: 500, width: '120px' }}>大小</th>
              <th style={{ padding: '10px 16px', fontWeight: 500, width: '180px' }}>更新时间</th>
              <th style={{ padding: '10px 16px', fontWeight: 500, width: '220px', textAlign: 'right' }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {files.length === 0 ? (
              <tr>
                <td colSpan={4} style={{ padding: '32px', textAlign: 'center', color: 'var(--text-secondary)' }}>
                  {loading ? "正在加载文件..." : "暂无已生成文件"}
                </td>
              </tr>
            ) : (
              files.map((f, i) => (
                <tr key={i} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td style={{ padding: '10px 16px', fontFamily: 'var(--font-mono)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <FileText size={14} color="var(--text-secondary)" />
                      <span>{f.name}</span>
                    </div>
                  </td>
                  <td style={{ padding: '10px 16px', fontFamily: 'var(--font-mono)', color: 'var(--text-secondary)' }}>
                    {f.size_human}
                  </td>
                  <td style={{ padding: '10px 16px', color: 'var(--text-secondary)', fontSize: '12px' }}>
                    {f.updated}
                  </td>
                  <td style={{ padding: '10px 16px', textAlign: 'right' }}>
                    <div style={{ display: 'inline-flex', gap: '6px' }}>
                      <button onClick={() => handlePreview(f)} className="btn-ghost" style={{ padding: '4px 8px', fontSize: '12px' }}>
                        <Eye size={12} /> 预览
                      </button>
                      <button onClick={() => handleCopy(f.raw_url)} className="btn-ghost" style={{ padding: '4px 8px', fontSize: '12px' }}>
                        <Copy size={12} /> 链接
                      </button>
                      <a href={f.raw_url} download style={{ textDecoration: 'none' }}>
                        <button style={{ padding: '4px 8px', fontSize: '12px' }}>
                          <Download size={12} /> 下载
                        </button>
                      </a>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* 极简在线代码预览弹窗 */}
      {previewFile && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, width: '100vw', height: '100vh',
          background: 'rgba(0,0,0,0.4)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 100000,
          padding: '24px'
        }}>
          <div style={{
            width: '100%',
            maxWidth: '900px',
            maxHeight: '85vh',
            background: '#ffffff',
            border: '1px solid var(--border-black)',
            display: 'flex',
            flexDirection: 'column'
          }}>
            <div style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '12px 16px',
              borderBottom: '1px solid var(--border)',
              background: 'var(--surface-1)'
            }}>
              <span style={{ fontSize: '14px', fontWeight: 500, fontFamily: 'var(--font-mono)' }}>{previewFile.name}</span>
              <button onClick={() => setPreviewFile(null)} className="btn-ghost" style={{ padding: '4px' }}>
                <X size={16} />
              </button>
            </div>
            <pre style={{
              flex: 1,
              overflowY: 'auto',
              padding: '16px',
              fontFamily: 'var(--font-mono)',
              fontSize: '12px',
              lineHeight: '1.6',
              background: '#fafafa',
              color: '#09090b',
              margin: 0
            }}>
              {previewFile.content}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
};
