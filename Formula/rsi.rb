class Rsi < Formula
  desc "Mihomo Manager CLI for Routers and Remote Control"
  homepage "https://github.com/WASIDJ/rsi"
  url "https://github.com/WASIDJ/rsi.git", branch: "main"
  version "1.0.0"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/rsi"
  end

  test do
    assert_match "RSI - Mihomo Manager CLI", shell_output("#{bin}/rsi --help")
  end
end
