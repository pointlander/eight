# Gesture recognition proof of concept
Image processing pipeline: video > background removal (optional) > descale > gray scale conversion > fft > nearest neighbor classification

## Building
```go
go build
```

## Usage
Select the video interface
```go
./eight -device <device>
```

There can be more than one point learned for a label.
```go
./eight -learn <label>
```

To recognize gestures: 
```go
./eight -infer
```

To test the video interface:
```go
./eight -picture
```

To use segmentation to remove background from frames:
```go
./eight -segmentation
```
(doesn't work that well)