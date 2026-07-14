use std::pin::Pin;
use std::task::{Context, Poll};
use tokio::io::{AsyncRead, ReadBuf};
use tracing::trace;

pub struct ChunkLimiter<R> {
    inner: R,
    max_chunk_size: usize,
}

impl<R: AsyncRead> ChunkLimiter<R> {
    pub fn new(inner: R, max_chunk_size: usize) -> Self {
        Self {
            inner,
            max_chunk_size,
        }
    }
}

impl<R: AsyncRead + Unpin> AsyncRead for ChunkLimiter<R> {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<std::io::Result<()>> {
        let limit = self.max_chunk_size.min(buf.remaining());

        let available = buf.initialize_unfilled_to(limit);
        let mut limited_buf = ReadBuf::new(available);

        let poll_result = Pin::new(&mut self.inner).poll_read(cx, &mut limited_buf);

        let filled_len = limited_buf.filled().len();
        trace!("Read {} bytes", filled_len);
        buf.advance(filled_len);

        poll_result
    }
}
