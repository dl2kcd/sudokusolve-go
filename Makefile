TARGET=sudokusolve

all: $(TARGET)

$(TARGET): $(TARGET).go
	go build $^

clean:
	rm -f $(TARGET)
