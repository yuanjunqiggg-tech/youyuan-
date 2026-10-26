package org.youyuan.toolbox;

import android.database.Cursor;
import android.database.MatrixCursor;
import android.os.CancellationSignal;
import android.os.ParcelFileDescriptor;
import android.provider.DocumentsContract.Document;
import android.provider.DocumentsProvider;
import android.webkit.MimeTypeMap;

import java.io.File;
import java.io.FileNotFoundException;
import java.util.Arrays;

/**
 * DocumentsProvider：把应用私有数据目录（getFilesDir）暴露给系统。
 * 第三方文件管理器在「存储分区/来源」里即可浏览、复制应用的全部私有数据（类似 Termux）。
 */
public class PrismDataProvider extends DocumentsProvider {

    private String authority() {
        return getContext().getPackageName() + ".documents";
    }

    private File getRootDir() {
        return getContext().getFilesDir();
    }

    private File getFileForDocId(String docId) {
        return new File(docId);
    }

    private String getDocIdForFile(File file) {
        return file.getAbsolutePath();
    }

    private void enforceInsideRoot(File file) throws FileNotFoundException {
        String root = getRootDir().getAbsolutePath();
        String path = file.getAbsolutePath();
        if (!path.equals(root) && !path.startsWith(root + File.separator)) {
            throw new FileNotFoundException("不允许访问路径: " + path);
        }
    }

    private String getMimeType(File file) {
        if (file.isDirectory()) return Document.MIME_TYPE_DIR;
        String name = file.getName();
        int dot = name.lastIndexOf('.');
        String ext = dot >= 0 ? name.substring(dot + 1) : "";
        String mime = MimeTypeMap.getSingleton().getMimeTypeFromExtension(ext);
        return mime != null ? mime : "application/octet-stream";
    }

    private void addRow(MatrixCursor result, File file) {
        MatrixCursor.RowBuilder row = result.newRow();
        row.add(Document.COLUMN_DOCUMENT_ID, getDocIdForFile(file));
        row.add(Document.COLUMN_DISPLAY_NAME, file.getName());
        row.add(Document.COLUMN_MIME_TYPE, getMimeType(file));
        if (file.isDirectory()) {
            row.add(Document.COLUMN_FLAGS, Document.FLAG_DIR_PREFERS_LAST_MODIFIED | Document.FLAG_DIR_SUPPORTS_CREATE);
        } else {
            row.add(Document.COLUMN_FLAGS, Document.FLAG_DIR_PREFERS_LAST_MODIFIED);
            row.add(Document.COLUMN_SIZE, file.length());
        }
        row.add(Document.COLUMN_LAST_MODIFIED, file.lastModified());
    }

    @Override
    public Cursor queryRoots(String[] projection) throws FileNotFoundException {
        MatrixCursor result = new MatrixCursor(projection != null ? projection : new String[]{
            Document.COLUMN_DOCUMENT_ID, Document.COLUMN_DISPLAY_NAME,
            Document.COLUMN_MIME_TYPE, Document.COLUMN_FLAGS
        });
        File root = getRootDir();
        MatrixCursor.RowBuilder row = result.newRow();
        row.add(Document.COLUMN_DOCUMENT_ID, getDocIdForFile(root));
        row.add(Document.COLUMN_DISPLAY_NAME, "游园工具箱数据");
        row.add(Document.COLUMN_MIME_TYPE, Document.MIME_TYPE_DIR);
        row.add(Document.COLUMN_FLAGS, Document.FLAG_DIR_PREFERS_LAST_MODIFIED);
        return result;
    }

    @Override
    public Cursor queryDocument(String documentId, String[] projection) throws FileNotFoundException {
        MatrixCursor result = new MatrixCursor(projection != null ? projection : new String[]{Document.COLUMN_DOCUMENT_ID});
        File file = getFileForDocId(documentId);
        enforceInsideRoot(file);
        addRow(result, file);
        return result;
    }

    @Override
    public Cursor queryChildDocuments(String parentDocumentId, String[] projection, String sortOrder)
            throws FileNotFoundException {
        File parent = getFileForDocId(parentDocumentId);
        enforceInsideRoot(parent);
        MatrixCursor result = new MatrixCursor(projection != null ? projection : new String[]{Document.COLUMN_DOCUMENT_ID});
        File[] children = parent.listFiles();
        if (children != null) {
            Arrays.sort(children);
            for (File child : children) {
                addRow(result, child);
            }
        }
        return result;
    }

    @Override
    public ParcelFileDescriptor openDocument(String documentId, String mode, CancellationSignal signal)
            throws FileNotFoundException {
        File file = getFileForDocId(documentId);
        enforceInsideRoot(file);
        int pfdMode;
        switch (mode) {
            case "r": pfdMode = ParcelFileDescriptor.MODE_READ_ONLY; break;
            case "w": pfdMode = ParcelFileDescriptor.MODE_WRITE_ONLY | ParcelFileDescriptor.MODE_TRUNCATE; break;
            case "rw": pfdMode = ParcelFileDescriptor.MODE_READ_WRITE | ParcelFileDescriptor.MODE_TRUNCATE; break;
            default: throw new FileNotFoundException("不支持的模式: " + mode);
        }
        return ParcelFileDescriptor.open(file, pfdMode);
    }

    @Override
    public String getDocumentType(String documentId) throws FileNotFoundException {
        return getMimeType(getFileForDocId(documentId));
    }

    @Override
    public boolean onCreate() {
        return true;
    }
}
