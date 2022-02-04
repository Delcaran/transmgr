docker build -t alpine/transmgr:latest .
docker run -v %cd%:/ext alpine/transmgr:latest cp /app/transmgr /ext