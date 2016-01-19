#include <stdlib.h>
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <errno.h>
#ifndef __MINGW32__
#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <arpa/inet.h>
#ifndef O_BINARY
#define O_BINARY 0
#endif

#else
#include <winsock2.h>
#include <ws2tcpip.h>
#ifndef EWOULDBLOCK
#define EWOULDBLOCK WSAEWOULDBLOCK
#endif
#ifndef EINPROGRESS
#define EINPROGRESS WSAEINPROGRESS
#endif
#define errno WSAGetLastError()
#define close(fd) closesocket(fd)
#endif

#ifndef SOL_TCP
#define SOL_TCP IPPROTO_TCP
#endif
#define NETWORK_TIMEOUT 60
#define DEFAULT_PORT 5000

//init win32 socket, just do nothing and return 0 for linux
int winsockInit(void)
{
#ifdef __MINGW32__
    WORD wVersionRequested;
    WSADATA wsaData;
    int ret;
    wVersionRequested = MAKEWORD(1, 1);
    ret               = WSAStartup(wVersionRequested, &wsaData);
    if (ret != 0) {
        return -1;
    }
    if (LOBYTE(wsaData.wVersion) != 1 || HIBYTE(wsaData.wVersion) != 1) {
        WSACleanup();
        return -1;
    }
#endif
    return 0;
}

void winsockCleanup(void) {
#ifdef __MINGW32__
    WSACleanup();
#endif
}

int setnonblocking(int fd)
{
#ifdef __MINGW32__
    u_long on = 1;
    return ioctlsocket(fd, FIONBIO, &on);
#else
    int flags;
    if (-1 == (flags = fcntl(fd, F_GETFL, 0))) {
        flags = 0;
    }
    return fcntl(fd, F_SETFL, flags | O_NONBLOCK);
#endif
}

int isCiaFile(const char *path) {
    int len = strlen(path);
    if(len < 4) {
        return 0;
    }
    const char *ext = &path[len - 4];
    if((ext[0] == '.') &&
        (ext[1] == 'c' || ext[1] == 'C') &&
        (ext[2] == 'i' || ext[2] == 'I') &&
        (ext[3] == 'a' || ext[3] == 'A')) {
        return 1;
    }
    return 0;
}

int strrchridx(const char *ipstr, int c) {
    int i;
    int len = strlen(ipstr);
    for(i = len-1; i >= 0; --i) {
        if(ipstr[i] == c) {
            return i;
        }
    }
    return -1;
}

//parse ipv4 adress and port
int parseIPPort(const char *ipstr, struct sockaddr_in *addr) {
    int idx = strrchridx(ipstr, ':');
    if(idx < 0) {
        addr->sin_family = AF_INET;
        addr->sin_port = htons(DEFAULT_PORT);
        addr->sin_addr.s_addr = inet_addr(ipstr);
    } else {
        char *tmp = strdup(ipstr);
        tmp[idx] = 0;
        addr->sin_family = AF_INET;
        int port = atoi(tmp+idx+1);
        addr->sin_addr.s_addr = inet_addr(tmp);
        free(tmp);
        if(port >= 65536 || port <= 0) {
            return -1;
        } 
        addr->sin_port = htons(port);
    }
    return 0;
}

int64_t hton64(int64_t host) {
    // test byte order
    short tmp = 0x0102;
    if(*(char *)&tmp == 0x01) { // big endian
        return host;
    } else { //little endian
        return (((int64_t)htonl(host))<<32) + htonl(host>>32); 
    }
    return host;
}

double timeDiff(struct timeval tv1, struct timeval tv2) {
    return (tv1.tv_sec - tv2.tv_sec) * 1.0 + (tv1.tv_usec - tv2.tv_usec) / 1000000.0;
}

ssize_t writen(int fd, const char *buf, size_t n, int timeout) {
    int nleft;
    int nwritten;
    nleft = n;
    struct timeval tv1, tv2;
    gettimeofday(&tv1, NULL);
    while(nleft > 0) {
        if((nwritten = send(fd, buf, nleft, 0)) <= 0) {
            if(nwritten < 0 && (errno == EWOULDBLOCK || errno == EAGAIN || errno == EINPROGRESS)){
                gettimeofday(&tv2, NULL);
                usleep(1000); // sleep 1ms, for slow network speed
                if(timeDiff(tv2, tv1) > (timeout * 1.0)) {
                    return -1;
                }
                continue;
            } else {
                return -1;
            }
        }
        gettimeofday(&tv1, NULL);
        nleft -= nwritten;
        buf += nwritten;
    }
    return n;
}

int main(int argc,char** argv) {
    int sockfd = -1;
    int ciafd = -1;
    int success = 0;
    struct sockaddr_in addr;

    printf("sockfilec 0.1\n");
    if(argc <= 2) {
        printf("Usage: %s <ip> <ciafile>\n", argv[0]);
        return -1;
    }

    // check cia file
    char *ciaPath = argv[2];
    if(!isCiaFile(ciaPath)) {
        printf("Not a cia file\n");
        return -1;
    }

    // check ip port
    if(parseIPPort(argv[1], &addr) < 0) {
        printf("Wrong ip format\n");
        return -1;
    }

    if(winsockInit() < 0) {
        printf("Win32 socket init failed\n");
        return -1;
    }

    // init socket
    sockfd = socket(AF_INET, SOCK_STREAM, 0);
    if(sockfd < 0) {
        printf("Failed creating socket: %s.\n", strerror(errno));
        goto end;
    } else {
        printf("Client socket created on port %d\n", ntohs(addr.sin_port));
    }

    int ret = connect(sockfd, (struct sockaddr*)&addr, sizeof(addr));
    if(ret < 0) { 
        printf("Failed connecting server: %s.\n", strerror(errno));
        goto end;
    }

    // O_BINARY needed by win
    ciafd = open(ciaPath, O_RDONLY | O_BINARY);

    if(ciafd < 0) {
        printf("File open failed: %s\n", strerror(errno));
        goto end;
    }

    struct stat st;
    fstat(ciafd, &st);
    int64_t filesize = st.st_size;
    if(filesize <= 0) {
        printf("Empty file\n");
        goto end;
    }

    // send file size
    int64_t filesizeNetorder = hton64(filesize);
    ret = send(sockfd, (char *)&filesizeNetorder, sizeof(filesizeNetorder), 0);
    if(ret != sizeof(int64_t)) {
        printf("Network error\n");
        goto end;
    }

    // prepare socket opt for file send
    setnonblocking(sockfd);
    // int opt = 1;
    // setsockopt(sockfd, SOL_TCP, TCP_NODELAY, (const char *)&opt, sizeof(opt));

    //send file
    char buf[1024*128];
    int bufsize = sizeof(buf);
    printf("Start intall file: %s\n", ciaPath);
    while(1) {
        int n = read(ciafd, buf, bufsize);
        if(n < 0) {
            printf("Read cia file error: %s\n", strerror(errno));
            goto end;
        }
        if(n == 0) {
            success = 1;
            break;
        }
        if(writen(sockfd, buf, n, NETWORK_TIMEOUT) < 0) {
            printf("Send cia file error: %s\n", strerror(errno));
            goto end;
        }
    }

end:
    if(sockfd >= 0) {
        close(sockfd);
    }
    if(ciafd >= 0) {
        close(ciafd);
    }
    winsockCleanup();
    if(success) {
        printf("Send cia file success\n");
        return 0;
    } else {
        printf("Send cia file failed\n");
        return -1;
    }
}
