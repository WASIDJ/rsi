class Rsi < Formula
  desc "Mihomo Manager CLI for Routers and Remote Control"
  homepage "https://github.com/WASIDJ/rsi"
  url "https://github.com/WASIDJ/rsi/archive/refs/tags/v1.1.0.tar.gz"
  version "1.1.0"
  sha256 "ef5729eebc34e3375e7236ed604160623bd4590df86f05a29f256b624f0df59f"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/rsi"
  end

  test do
    assert_match "RSI - Mihomo Manager CLI", shell_output("#{bin}/rsi --help")
  end
end
