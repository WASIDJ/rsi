class Rsi < Formula
  desc "Mihomo Manager CLI for Routers and Remote Control"
  homepage "https://github.com/WASIDJ/rsi"
  url "https://github.com/WASIDJ/rsi/archive/refs/tags/v1.0.0.tar.gz"
  version "1.0.0"
  sha256 "a0bf68ce7da158867ea4d425e68073b71568bb21c4402dfb449b177c6e419784"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/rsi"
  end

  test do
    assert_match "RSI - Mihomo Manager CLI", shell_output("#{bin}/rsi --help")
  end
end
