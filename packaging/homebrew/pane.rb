class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.2"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.2/pane-v0.1.2-darwin-arm64.tar.gz"
      sha256 "48a5579b01c0ae8a2172488273f7151d29c2707e4f439c9c041d502fb7ba906c"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.2/pane-v0.1.2-darwin-amd64.tar.gz"
      sha256 "77be16bc34d1032665eb1e880d4ede3a929b46f90b0501603e8a4b8ff1043d98"
    end
  end

  depends_on "go" => :build if build.head?
  depends_on "rust" => :build if build.head?

  def install
    if build.head?
      ENV.prepend_path "PATH", Formula["rust"].opt_bin
      system "make", "build"
      bin.install "bin/pane"
      bin.install "bin/pane-analyze"
    else
      bin.install "pane"
      bin.install "pane-analyze"
    end
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
